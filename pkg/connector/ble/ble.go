// Package ble implements the vehicle.Connector interface using BLE.

package ble

import (
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/teslamotors/vehicle-command/internal/log"
	"github.com/teslamotors/vehicle-command/pkg/connector"
	"github.com/teslamotors/vehicle-command/pkg/protocol"
	"tinygo.org/x/bluetooth"
)

const maxBLEMessageSize = 1024

var ErrMaxConnectionsExceeded = protocol.NewError("the vehicle is already connected to the maximum number of BLE devices", false, false)

var (
	rxTimeout  = time.Second     // Timeout interval between receiving chunks of a mesasge
	maxLatency = 4 * time.Second // Max allowed error when syncing vehicle clock
)

func mustParseUUID(uuid string) bluetooth.UUID {
	uuidParsed, err := bluetooth.ParseUUID(uuid)
	if err != nil {
		panic(err)
	}
	return uuidParsed
}

var (
	vehicleServiceUUID = mustParseUUID("00000211-b2d1-43f0-9b88-960cebf8b91e")
	toVehicleUUID      = mustParseUUID("00000212-b2d1-43f0-9b88-960cebf8b91e")
	fromVehicleUUID    = mustParseUUID("00000213-b2d1-43f0-9b88-960cebf8b91e")
)

var (
	adapter *bluetooth.Adapter
	mu      sync.Mutex
)

type Connection struct {
	vin         string
	inbox       chan []byte
	txChar      bluetooth.DeviceCharacteristic
	blockLength int
	rxChar      bluetooth.DeviceCharacteristic
	inputBuffer []byte
	device      bluetooth.Device
	lastRx      time.Time
	lock        sync.Mutex
}

func (c *Connection) PreferredAuthMethod() connector.AuthMethod {
	return connector.AuthMethodGCM
}

func (c *Connection) RetryInterval() time.Duration {
	return time.Second
}

func (c *Connection) Receive() <-chan []byte {
	return c.inbox
}

func (c *Connection) flush() bool {
	if len(c.inputBuffer) >= 2 {
		msgLength := 256*int(c.inputBuffer[0]) + int(c.inputBuffer[1])
		if msgLength > maxBLEMessageSize {
			c.inputBuffer = []byte{}
			return false
		}
		if len(c.inputBuffer) >= 2+msgLength {
			buffer := c.inputBuffer[2 : 2+msgLength]
			log.Debug("RX: %02x", buffer)
			c.inputBuffer = c.inputBuffer[2+msgLength:]
			select {
			case c.inbox <- buffer:
			default:
				return false
			}
			return true
		}
	}
	return false
}

func (c *Connection) Close() {
	if err := c.rxChar.EnableNotifications(nil); err != nil {
		log.Warning("ble: failed to disable RX notifications: %s", err)
	}
	if err := c.device.Disconnect(); err != nil {
		log.Warning("ble: failed to disconnect: %s", err)
	}
}

func (c *Connection) AllowedLatency() time.Duration {
	return maxLatency
}

func (c *Connection) rx(p []byte) {
	if time.Since(c.lastRx) > rxTimeout {
		c.inputBuffer = []byte{}
	}
	c.lastRx = time.Now()
	c.inputBuffer = append(c.inputBuffer, p...)
	for c.flush() {
	}
}

func (c *Connection) Send(_ context.Context, buffer []byte) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	var out []byte
	log.Debug("TX: %02x", buffer)
	out = append(out, uint8(len(buffer)>>8), uint8(len(buffer)))
	out = append(out, buffer...)
	blockLength := c.blockLength
	for len(out) > 0 {
		if blockLength > len(out) {
			blockLength = len(out)
		}

		n, err := c.txChar.WriteWithoutResponse(out[:blockLength])
		if err != nil {
			return err
		} else if n != blockLength {
			return fmt.Errorf("ble: failed to write %d bytes", blockLength)
		}
		out = out[blockLength:]
	}
	return nil
}

func (c *Connection) VIN() string {
	return c.vin
}

func VehicleLocalName(vin string) string {
	vinBytes := []byte(vin)
	digest := sha1.Sum(vinBytes)
	return fmt.Sprintf("S%02xC", digest[:8])
}

// InitAdapterWithID initializes the BLE adapter with the given ID.
// Currently this is only supported on Linux. It is not necessary to
// call this function if using the default adapter, but if not, it
// must be called before making any other BLE calls.
// Linux:
//   - id is in the form "hciX" where X is the number of the adapter.
func InitAdapterWithID(id string) error {
	mu.Lock()
	defer mu.Unlock()
	return initAdapter(&id)
}

// CloseAdapter unsets the BLE adapter so that a new one can be created
// on the next call to InitAdapter. This does not disconnect any existing
// connections or stop any ongoing scans and must be done separately.
func CloseAdapter() error {
	mu.Lock()
	defer mu.Unlock()
	adapter = nil
	return nil
}

func initAdapter(id *string) error {
	var err error
	if adapter != nil {
		log.Debug("Reusing existing BLE device")
	} else {
		log.Debug("Creating new BLE adapter")
		idStr := ""
		if id != nil {
			idStr = *id
		}
		adapter = newAdapter(idStr)
		if err = adapter.Enable(); err != nil {
			return fmt.Errorf("ble: failed to enable device: %s", err)
		}
	}
	return nil
}

type ScanResult struct {
	Address   bluetooth.Address
	LocalName string
	RSSI      int16
}

func ScanVehicleBeacon(ctx context.Context, vin string) (*ScanResult, error) {
	mu.Lock()
	defer mu.Unlock()

	if err := initAdapter(nil); err != nil {
		return nil, err
	}

	a, err := scanVehicleBeacon(ctx, VehicleLocalName(vin))
	if err != nil {
		return nil, fmt.Errorf("ble: failed to scan for %s: %s", vin, err)
	}
	return a, nil
}

func scanVehicleBeacon(ctx context.Context, localName string) (*ScanResult, error) {
	scanIsStopped := false
	stopScan := func() {
		scanIsStopped = true
		if err := adapter.StopScan(); err != nil {
			log.Warning("ble: failed to stop scan: %s", err)
		}
	}

	errorCh := make(chan error)
	foundCh := make(chan *ScanResult)

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
		if err := adapter.Scan(func(_ *bluetooth.Adapter, result bluetooth.ScanResult) {
			// If we have stopped the scan and we still get results it means
			// that the case described in the comment above has happened.
			if scanIsStopped {
				stopScan()
				return
			}

			if result.LocalName() == localName {
				stopScan()
				foundCh <- &ScanResult{
					Address:   result.Address,
					LocalName: result.LocalName(),
					RSSI:      result.RSSI,
				}
			}
		}); err != nil {
			errorCh <- err
		}
	}()

	select {
	case result := <-foundCh:
		return result, nil
	case err := <-errorCh:
		return nil, err
	case <-ctx.Done():
		stopScan()
		return nil, ctx.Err()
	}
}

func NewConnection(ctx context.Context, vin string) (*Connection, error) {
	return NewConnectionFromScanResult(ctx, vin, nil)
}

// NewConnectionFromScanResult creates a new BLE connection to the given target.
// If target is nil, the vehicle will be scanned for.
//
// NOTE(Linux/bluez): If target is specified the user must make sure that the
// time between scanning and connecting is no longer than ~10 seconds as if
// it is, bluez will not allow the connection to be established until it is
// rescanned.
func NewConnectionFromScanResult(ctx context.Context, vin string, target *ScanResult) (*Connection, error) {
	var lastError error
	for {
		conn, retry, err := tryToConnect(ctx, vin, target)
		if err == nil {
			return conn, nil
		}
		if !retry || IsAdapterError(err) {
			return nil, err
		}
		log.Warning("BLE connection attempt failed: %s", err)
		if err := ctx.Err(); err != nil {
			if lastError != nil {
				return nil, lastError
			}
			return nil, err
		}
		lastError = err
	}
}

func tryToConnect(ctx context.Context, vin string, target *ScanResult) (*Connection, bool, error) {
	var err error
	mu.Lock()
	defer mu.Unlock()

	if err = initAdapter(nil); err != nil {
		return nil, false, err
	}

	localName := VehicleLocalName(vin)

	if target == nil {
		target, err = scanVehicleBeacon(ctx, localName)
		if err != nil {
			return nil, true, fmt.Errorf("ble: failed to scan for %s: %s", vin, err)
		}
	}

	if target.LocalName != localName {
		return nil, false, fmt.Errorf("ble: beacon with unexpected local name: '%s'", target.LocalName)
	}

	log.Debug("Connecting to %s (%s)...", target.Address.String(), localName)

	// FIXME: This is a workaround for the fact that bluetooth library doesn't
	// support context handling. While not a big issue, it will be good to
	// have this fixed upstream. Also applies to the DiscoverServices and
	// DiscoverCharacteristics calls below. See:
	// https://github.com/tinygo-org/bluetooth/issues/339
	deviceCh := make(chan bluetooth.Device)
	errorCh := make(chan error)
	connectionCancelled := false
	go func() {
		params := bluetooth.ConnectionParams{}
		// If a deadline is set, use it as the connection timeout
		// else this go routine will expire after the default timeout.
		if deadline, ok := ctx.Deadline(); ok {
			params.ConnectionTimeout = bluetooth.NewDuration(time.Until(deadline))
		}
		device, err := adapter.Connect(target.Address, params)
		if err != nil && !connectionCancelled {
			errorCh <- err
		} else if !connectionCancelled {
			deviceCh <- device
		} else {
			if err := device.Disconnect(); err != nil {
				log.Warning("ble: failed to disconnect: %s", err)
			}
		}
	}()
	var device bluetooth.Device
	select {
	case device = <-deviceCh:
		log.Debug("Connected to %s", target.Address.String())
	case err := <-errorCh:
		return nil, true, fmt.Errorf("ble: failed to connect to device: %s", err)
	case <-ctx.Done():
		connectionCancelled = true
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
		log.Warning("Failed to get TX MTU (using 20): %s", err)
		mtu = 20
	}

	conn := Connection{
		vin:         vin,
		device:      device,
		inbox:       make(chan []byte, 5),
		txChar:      characteristics[0],
		rxChar:      characteristics[1],
		blockLength: int(mtu) - 3,
	}

	if err := conn.rxChar.EnableNotifications(conn.rx); err != nil {
		return nil, true, fmt.Errorf("ble: failed to subscribe to RX: %s", err)
	}
	log.Info("Connected to vehicle BLE")
	return &conn, false, nil
}
