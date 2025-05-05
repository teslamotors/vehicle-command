package tinygo

import (
	"context"
	"errors"
	"fmt"
	"github.com/teslamotors/vehicle-command/pkg/connector/ble/iface"
	"sync"
	"time"

	"github.com/teslamotors/vehicle-command/internal/log"

	"tinygo.org/x/bluetooth"
)

var (
	vehicleServiceUUID = mustParseUUID(iface.VehicleServiceUUID)
	toVehicleUUID      = mustParseUUID(iface.ToVehicleUUID)
	fromVehicleUUID    = mustParseUUID(iface.FromVehicleUUID)
)

var (
	device *bluetooth.Adapter
	mu     sync.Mutex
)

func NewAdapter() iface.Adapter {
	return adapter{}
}

type adapter struct{}

func (a adapter) AdapterErrorHelpMessage(err error) string {
	return AdapterErrorHelpMessage(err)
}

func (a adapter) InitAdapter(id string) error {
	mu.Lock()
	defer mu.Unlock()

	if device != nil {
		log.Debug("Reusing existing BLE device")
	} else {
		log.Debug("Creating new BLE adapter")

		var err error
		device, err = newAdapter(id)
		if err != nil {
			return fmt.Errorf("ble: failed to enable device: %s", err)
		}
		if err = device.Enable(); err != nil {
			return fmt.Errorf("ble: failed to enable device: %s", err)
		}
	}
	return nil
}

func (a adapter) CloseAdapter() error {
	mu.Lock()
	defer mu.Unlock()
	device = nil
	return nil
}

func (a adapter) ScanVehicleBeacon(ctx context.Context, localName string) (*iface.ScanResult, error) {
	scanIsStopped := false
	stopScan := func() {
		scanIsStopped = true
		if err := device.StopScan(); err != nil {
			log.Warning("ble: failed to stop scan: %s", err)
		}
	}

	errorCh := make(chan error)
	foundCh := make(chan *iface.ScanResult)

	// FIXME: We need to check if the context is already done to prevent starting the scan
	// and then immediately stopping it, since that would create a chance for
	// the scan to not be stopped before being started. There is still a small chance that
	// the scan will not be stopped properly if the context is done between
	// the check and the start of the scan, but that is a very small window.
	// This can be fixed once the bluetooth library supports context handling.
	// See: https://github.com/tinygo-org/bluetooth/issues/339
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Another fix: We need to wait for the Scan() call to finish before returning because
	// waiting for StopScan() to finish is not enough.
	scanFinished := make(chan struct{})
	defer func() {
		<-scanFinished
	}()

	// Scan is blocking, so run it in a goroutine
	go func() {
		defer close(scanFinished)
		if scanIsStopped {
			return
		}
		log.Debug("Scanning for %s...", localName)
		if err := device.Scan(func(_ *bluetooth.Adapter, result bluetooth.ScanResult) {
			// If we have stopped the scan, and we still get results it means
			// that the case described in the comment above has happened.
			if scanIsStopped {
				stopScan()
				return
			}

			if result.LocalName() == localName {
				stopScan()
				foundCh <- &iface.ScanResult{
					Address:     result.Address.String(),
					LocalName:   result.LocalName(),
					RSSI:        result.RSSI,
					Connectable: true,
				}
			}
		}); err != nil && !scanIsStopped {
			errorCh <- err
		}
	}()

	select {
	case result := <-foundCh:
		return result, nil
	case err := <-errorCh:
		// FIX: If we get an error we can not be sure that the scan has stopped
		// so we need to stop it manually.
		// See: https://github.com/tinygo-org/bluetooth/issues/340
		stopScan()
		return nil, err
	case <-ctx.Done():
		stopScan()
		return nil, ctx.Err()
	}
}

func (a adapter) TryToConnect(ctx context.Context, vin string, target *iface.ScanResult) (*iface.Connection, bool, error) {
	log.Debug("Connecting to %s (%s)...", target.Address, target.LocalName)

	// FIXME: This is a workaround for the fact that bluetooth library doesn't
	// support context handling. While not a big issue, it will be good to
	// have this fixed upstream. Also applies to the DiscoverServices and
	// DiscoverCharacteristics calls below. See:
	// https://github.com/tinygo-org/bluetooth/issues/339
	deviceCh := make(chan bluetooth.Device, 1)
	errorCh := make(chan error, 1)
	go func() {
		params := bluetooth.ConnectionParams{}
		// If a deadline is set, use it as the connection timeout
		// else this go routine will expire after the default timeout.
		if deadline, ok := ctx.Deadline(); ok {
			params.ConnectionTimeout = bluetooth.NewDuration(time.Until(deadline))
		}

		addr, err := parseAddress(target.Address)
		if err != nil {
			errorCh <- err
			return
		}

		device, err := device.Connect(addr, params)
		if err != nil {
			errorCh <- err
			return
		}
		if ctx.Err() == nil {
			deviceCh <- device
			return
		}
		if err := device.Disconnect(); err != nil {
			log.Warning("ble: failed to disconnect: %s", err)

		}
	}()
	var device bluetooth.Device
	select {
	case device = <-deviceCh:
		log.Debug("Connected to %s", target.Address)
	case err := <-errorCh:
		return nil, true, fmt.Errorf("ble: failed to connect to device: %s", err)
	case <-ctx.Done():
		return nil, true, ctx.Err()
	}

	log.Debug("Discovering services %s...", device.Address.String())
	services, err := device.DiscoverServices([]bluetooth.UUID{vehicleServiceUUID})
	if err != nil {
		return nil, true, fmt.Errorf("ble: failed to enumerate device services: %s", err)
	}
	if len(services) != 1 {
		return nil, true, fmt.Errorf("ble: failed to discover service")
	}

	characteristics, err := services[0].DiscoverCharacteristics([]bluetooth.UUID{toVehicleUUID, fromVehicleUUID})
	if err != nil {
		return nil, true, fmt.Errorf("ble: failed to discover service characteristics: %s", err)
	}

	if len(characteristics) != 2 {
		return nil, true, errors.New("ble: failed to find required characteristics")
	}

	mtu, err := characteristics[0].GetMTU()
	if err != nil {
		log.Warning("Failed to get TX MTU (using 23): %s", err)
		mtu = 23
	}

	blockLength := int(mtu) - 3
	w := &writer{
		txChar: characteristics[0],
		rxChar: characteristics[1],
		device: device,
	}
	connection := iface.NewConnection(vin, blockLength, w)

	if err := w.rxChar.EnableNotifications(connection.Rx); err != nil {
		return nil, true, fmt.Errorf("ble: failed to subscribe to RX: %s", err)
	}
	log.Info("Connected to vehicle BLE")
	return connection, false, nil
}

func (a adapter) IsAdapterError(err error) bool {
	return IsAdapterError(err)
}

type writer struct {
	txChar bluetooth.DeviceCharacteristic
	rxChar bluetooth.DeviceCharacteristic
	device bluetooth.Device
}

func (w *writer) WriteCharacteristic(bytes []byte, length int) error {
	n, err := deviceCharacteristicWrite(w.txChar, bytes)
	if err != nil {
		return err
	}
	if n != length {
		return fmt.Errorf("ble: failed to write %d bytes", length)
	}

	return nil
}

func parseAddress(address string) (bluetooth.Address, error) {
	mac, err := bluetooth.ParseMAC(address)
	if err != nil {

		return bluetooth.Address{}, fmt.Errorf("ble: failed to parse MAC address: %s", err)
	}

	return bluetooth.Address{
		MACAddress: bluetooth.MACAddress{
			MAC: mac,
		},
	}, nil
}

func mustParseUUID(uuid string) bluetooth.UUID {
	uuidParsed, err := bluetooth.ParseUUID(uuid)
	if err != nil {
		panic(err)
	}
	return uuidParsed
}
