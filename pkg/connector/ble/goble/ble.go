package goble

import (
	"context"
	"errors"
	"fmt"
	"github.com/teslamotors/vehicle-command/internal/log"
	"github.com/teslamotors/vehicle-command/pkg/connector/ble/iface"
	"github.com/zlymeda/go-ble"
	"sync"
)

var (
	vehicleServiceUUID = ble.MustParse(iface.VehicleServiceUUID)
	toVehicleUUID      = ble.MustParse(iface.ToVehicleUUID)
	fromVehicleUUID    = ble.MustParse(iface.FromVehicleUUID)
)

var (
	device ble.Device
	mu     sync.Mutex
)

func NewAdapter() iface.Adapter {
	return adapter{}
}

type adapter struct {
}

func (a adapter) AdapterErrorHelpMessage(err error) string {
	return AdapterErrorHelpMessage(err)
}

func (a adapter) InitAdapter(id string) error {
	mu.Lock()
	defer mu.Unlock()

	var err error
	// We don't want concurrent calls to NewConnection that would defeat
	// the point of reusing the existing BLE device. Note that this is not
	// an issue on MacOS, but multiple calls to newDevice() on Linux leads to failures.
	if device != nil {
		log.Debug("Reusing existing BLE device")
	} else {
		log.Debug("Creating new BLE adapter")
		device, err = newAdapter(id)
		if err != nil {
			return fmt.Errorf("ble: failed to enable device: %s", err)
		}
	}
	return nil
}

// CloseAdapter unsets the BLE adapter so that a new one can be created
// on the next call to InitAdapter. This does not disconnect any existing
// connections or stop any ongoing scans and must be done separately.
func (a adapter) CloseAdapter() error {
	mu.Lock()
	defer mu.Unlock()
	if device != nil {
		if err := device.Stop(); err != nil {
			return fmt.Errorf("ble: failed to stop device: %s", err)
		}
		device = nil
		log.Debug("Closed BLE adapter")
	}
	return nil
}

func (a adapter) ScanVehicleBeacon(ctx context.Context, localName string) (*iface.ScanResult, error) {
	var err error
	ctx2, cancel := context.WithCancel(ctx)
	defer cancel()

	ch := make(chan ble.Advertisement, 1)
	fn := func(a ble.Advertisement) {
		if a.LocalName() != localName {
			return
		}
		select {
		case ch <- a:
			cancel() // Notify device.Scan() that we found a match
		case <-ctx2.Done():
			// Another goroutine already found a matching advertisement. We need to return so that
			// the MacOS implementation of device.Scan(...) unblocks.
		}
	}

	if err = device.Scan(ctx2, false, fn); !errors.Is(err, context.Canceled) {
		// If ctx rather than ctx2 was canceled, we'll pick that error up below. This is a bit
		// hacky, but unfortunately device.Scan() _always_ returns an error on MacOS because it does
		// not terminate until the provided context is canceled.
		return nil, err
	}

	select {
	case a, ok := <-ch:
		if !ok {
			// This should never happen, but just in case
			return nil, fmt.Errorf("scan channel closed")
		}
		return advertisementToScanResult(a), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (a adapter) TryToConnect(ctx context.Context, vin string, target *iface.ScanResult) (*iface.Connection, bool, error) {
	log.Debug("Dialing to %s (%s)...", target.Address, target.LocalName)

	client, err := device.Dial(ctx, ble.NewAddr(target.Address))
	if err != nil {
		return nil, true, fmt.Errorf("ble: failed to dial for %s (%s): %s", vin, target.LocalName, err)
	}

	log.Debug("Discovering services %s...", client.Addr())
	services, err := client.DiscoverServices([]ble.UUID{vehicleServiceUUID})
	if err != nil {
		return nil, true, fmt.Errorf("ble: failed to enumerate device services: %s", err)
	}
	if len(services) == 0 {
		return nil, true, fmt.Errorf("ble: failed to discover service")
	}

	characteristics, err := client.DiscoverCharacteristics([]ble.UUID{toVehicleUUID, fromVehicleUUID}, services[0])
	if err != nil {
		return nil, true, fmt.Errorf("ble: failed to discover service characteristics: %s", err)
	}

	var txChar *ble.Characteristic
	var rxChar *ble.Characteristic

	for _, characteristic := range characteristics {
		if characteristic.UUID.Equal(toVehicleUUID) {
			txChar = characteristic
		} else if characteristic.UUID.Equal(fromVehicleUUID) {
			rxChar = characteristic
		}
		if _, err := client.DiscoverDescriptors(nil, characteristic); err != nil {
			return nil, true, fmt.Errorf("ble: couldn't fetch descriptors: %s", err)
		}
	}

	if txChar == nil || rxChar == nil {
		return nil, true, fmt.Errorf("ble: failed to find required characteristics")
	}

	connection := iface.NewConnection(vin, 0, writer{
		txChar: txChar,
		rxChar: rxChar,
		client: client,
	})

	if err := client.Subscribe(rxChar, true, connection.Rx); err != nil {
		return nil, true, fmt.Errorf("ble: failed to subscribe to RX: %s", err)
	}

	txMtu, err := client.ExchangeMTU(ble.MaxMTU)
	if err != nil {
		log.Warning("ble: failed to exchange MTU: %s", err)
		connection.SetBlockLength(ble.DefaultMTU - 3) // Fallback to default MTU size
	} else {
		connection.SetBlockLength(min(txMtu, iface.MaxBLEMessageSize) - 3) // 3 bytes for header
		log.Debug("MTU size: %d", txMtu)
	}

	log.Info("Connected to vehicle BLE")
	return connection, false, nil
}

func (a adapter) IsAdapterError(err error) bool {
	return IsAdapterError(err)
}

type writer struct {
	txChar *ble.Characteristic
	rxChar *ble.Characteristic
	client ble.Client
}

func (a writer) Close() {
	_ = a.client.ClearSubscriptions()
	_ = a.client.CancelConnection()
}

func (a writer) WriteCharacteristic(_ context.Context, bytes []byte) error {
	return a.client.WriteCharacteristic(a.txChar, bytes, false)
}

func advertisementToScanResult(a ble.Advertisement) *iface.ScanResult {
	return &iface.ScanResult{
		Address:     a.Addr().String(),
		LocalName:   a.LocalName(),
		RSSI:        int16(a.RSSI()),
		Connectable: a.Connectable(),
	}
}
