// Package ble implements the vehicle.Connector interface using BLE.

package ble

import (
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-ble/ble"
	"github.com/teslamotors/vehicle-command/internal/log"
	"github.com/teslamotors/vehicle-command/pkg/connector"
	"github.com/teslamotors/vehicle-command/pkg/protocol"
)

const (
	maxBLEMTUSize     = ble.MaxMTU // Max MTU size accepted by the client (this library)
	maxBLEMessageSize = 1024
)

var ErrAdapterInvalidID = protocol.NewError("the bluetooth adapter ID is invalid", false, false)
var ErrMaxConnectionsExceeded = protocol.NewError("the vehicle is already connected to the maximum number of BLE devices", false, false)

var (
	rxTimeout  = time.Second     // Timeout interval between receiving chunks of a mesasge
	maxLatency = 4 * time.Second // Max allowed error when syncing vehicle clock
)

var (
	vehicleServiceUUID = ble.MustParse("00000211-b2d1-43f0-9b88-960cebf8b91e")
	toVehicleUUID      = ble.MustParse("00000212-b2d1-43f0-9b88-960cebf8b91e")
	fromVehicleUUID    = ble.MustParse("00000213-b2d1-43f0-9b88-960cebf8b91e")
)

var (
	device ble.Device
	mu     sync.Mutex
)

type Connection struct {
	vin         string
	inbox       chan []byte
	txChar      *ble.Characteristic
	blockLength int
	rxChar      *ble.Characteristic
	inputBuffer []byte
	client      ble.Client
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
	_ = c.client.ClearSubscriptions()
	_ = c.client.CancelConnection()
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
		if err := c.client.WriteCharacteristic(c.txChar, out[:blockLength], false); err != nil {
			return err
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
	if device != nil {
		if err := device.Stop(); err != nil {
			return fmt.Errorf("ble: failed to stop device: %s", err)
		}
		device = nil
		log.Debug("Closed BLE adapter")
	}
	return nil
}

func initAdapter(id *string) error {
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

type ScanResult struct {
	Address     string
	LocalName   string
	RSSI        int16
	Connectable bool
}

func advertisementToScanResult(a ble.Advertisement) *ScanResult {
	return &ScanResult{
		Address:     a.Addr().String(),
		LocalName:   a.LocalName(),
		RSSI:        int16(a.RSSI()),
		Connectable: a.Connectable(),
	}
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

func NewConnection(ctx context.Context, vin string) (*Connection, error) {
	return NewConnectionFromScanResult(ctx, vin, nil)
}

// NewConnectionFromScanResult creates a new BLE connection to the given target.
// If target is nil, the vehicle will be scanned for.
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

	if !target.Connectable {
		return nil, false, ErrMaxConnectionsExceeded
	}

	log.Debug("Dialing to %s (%s)...", target.Address, localName)

	client, err := device.Dial(ctx, ble.NewAddr(target.Address))
	if err != nil {
		return nil, true, fmt.Errorf("ble: failed to dial for %s (%s): %s", vin, localName, err)
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

	conn := Connection{
		vin:    vin,
		client: client,
		inbox:  make(chan []byte, 5),
	}
	for _, characteristic := range characteristics {
		if characteristic.UUID.Equal(toVehicleUUID) {
			conn.txChar = characteristic
		} else if characteristic.UUID.Equal(fromVehicleUUID) {
			conn.rxChar = characteristic
		}
		if _, err := client.DiscoverDescriptors(nil, characteristic); err != nil {
			return nil, true, fmt.Errorf("ble: couldn't fetch descriptors: %s", err)
		}
	}
	if conn.txChar == nil || conn.rxChar == nil {
		return nil, true, fmt.Errorf("ble: failed to find required characteristics")
	}
	if err := client.Subscribe(conn.rxChar, true, conn.rx); err != nil {
		return nil, true, fmt.Errorf("ble: failed to subscribe to RX: %s", err)
	}

	txMtu, err := client.ExchangeMTU(maxBLEMTUSize)
	if err != nil {
		log.Warning("ble: failed to exchange MTU: %s", err)
		conn.blockLength = ble.DefaultMTU - 3 // Fallback to default MTU size
	} else {
		conn.blockLength = min(txMtu, maxBLEMessageSize) - 3 // 3 bytes for header
		log.Debug("MTU size: %d", txMtu)
	}

	log.Info("Connected to vehicle BLE")
	return &conn, false, nil
}
