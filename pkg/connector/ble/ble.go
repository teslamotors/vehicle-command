// Package ble implements the vehicle.Connector interface using BLE.

package ble

import (
	"context"
	"crypto/sha1"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-ble/ble"
	"github.com/teslamotors/vehicle-command/internal/log"
	"github.com/teslamotors/vehicle-command/pkg/connector"
	"github.com/teslamotors/vehicle-command/pkg/protocol"
)

const maxBLEMessageSize = 1024

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
	blockLength := 20
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

func initDevice() error {
	var err error
	// We don't want concurrent calls to NewConnection that would defeat
	// the point of reusing the existing BLE device. Note that this is not
	// an issue on MacOS, but multiple calls to newDevice() on Linux leads to failures.
	if device != nil {
		log.Debug("Reusing existing BLE device")
	} else {
		log.Debug("Creating new BLE device")
		device, err = newDevice()
		if err != nil {
			return fmt.Errorf("failed to find a BLE device: %s", err)
		}
		ble.SetDefaultDevice(device)
	}
	return nil
}

type Advertisement = ble.Advertisement

func ScanVehicleBeacon(ctx context.Context, vin string) (Advertisement, error) {
	mu.Lock()
	defer mu.Unlock()

	if err := initDevice(); err != nil {
		return nil, err
	}

	a, err := scanVehicleBeacon(ctx, VehicleLocalName(vin))
	if err != nil {
		return nil, fmt.Errorf("ble: failed to scan for %s: %s", vin, err)
	}
	return a, nil
}

func scanVehicleBeacon(ctx context.Context, localName string) (Advertisement, error) {
	var err error
	ctx2, cancel := context.WithCancel(ctx)

	ch := make(chan Advertisement)
	fn := func(a Advertisement) {
		if a.LocalName() != localName {
			return
		}
		cancel()
		ch <- a
	}
	err = device.Scan(ctx2, false, fn)

	select {
	case a, ok := <-ch:
		if !ok {
			// This should never happen, but just in case
			return nil, fmt.Errorf("scan channel closed")
		}
		return a, nil
	default:
		return nil, err
	}
}

func NewConnection(ctx context.Context, vin string) (*Connection, error) {
	return NewConnectionToBleTarget(ctx, vin, nil)
}

func NewConnectionToBleTarget(ctx context.Context, vin string, target Advertisement) (*Connection, error) {
	var lastError error
	for {
		conn, err, retry := tryToConnect(ctx, vin, target)
		if err == nil {
			return conn, nil
		}
		if !retry || strings.Contains(err.Error(), "operation not permitted") {
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

func tryToConnect(ctx context.Context, vin string, target Advertisement) (*Connection, error, bool) {
	var err error
	mu.Lock()
	defer mu.Unlock()

	if err = initDevice(); err != nil {
		return nil, err, false
	}

	localName := VehicleLocalName(vin)

	if target == nil {
		target, err = scanVehicleBeacon(ctx, localName)
		if err != nil {
			return nil, fmt.Errorf("ble: failed to scan for %s: %s", vin, err), true
		}
	}

	if target.LocalName() != localName {
		return nil, fmt.Errorf("ble: beacon with unexpected local name: '%s'", target.LocalName()), false
	}

	if !target.Connectable() {
		return nil, ErrMaxConnectionsExceeded, false
	}

	log.Debug("Dialing to %s (%s)...", target.Addr(), localName)

	client, err := ble.Dial(ctx, target.Addr())
	if err != nil {
		return nil, fmt.Errorf("ble: failed to dial for %s (%s): %s", vin, localName, err), true
	}

	log.Debug("Discovering services %s...", client.Addr())
	services, err := client.DiscoverServices([]ble.UUID{vehicleServiceUUID})
	if err != nil {
		return nil, fmt.Errorf("ble: failed to enumerate device services: %s", err), true
	}
	if len(services) == 0 {
		return nil, fmt.Errorf("ble: failed to discover service"), true
	}

	characteristics, err := client.DiscoverCharacteristics([]ble.UUID{toVehicleUUID, fromVehicleUUID}, services[0])
	if err != nil {
		return nil, fmt.Errorf("ble: failed to discover service characteristics: %s", err), true
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
			return nil, fmt.Errorf("ble: couldn't fetch descriptors: %s", err), true
		}
	}
	if conn.txChar == nil || conn.rxChar == nil {
		return nil, fmt.Errorf("ble: failed to find required characteristics"), true
	}
	if err := client.Subscribe(conn.rxChar, true, conn.rx); err != nil {
		return nil, fmt.Errorf("ble: failed to subscribe to RX: %s", err), true
	}
	log.Info("Connected to vehicle BLE")
	return &conn, nil, false
}
