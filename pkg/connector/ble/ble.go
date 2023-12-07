// Package ble implements the vehicle.Connector interface using BLE.

package ble

import (
	"context"
	"crypto/sha1"
	"fmt"
	"sync"
	"time"

	"github.com/go-ble/ble"
	"github.com/teslamotors/vehicle-command/internal/log"
	"github.com/teslamotors/vehicle-command/pkg/connector"
)

const maxBLEMessageSize = 1024

var (
	rxTimeout = time.Second
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
	c.client.ClearSubscriptions()
	c.client.CancelConnection()
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

func (c *Connection) Send(ctx context.Context, buffer []byte) error {
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

func NewConnection(ctx context.Context, vin string) (*Connection, error) {
	var err error
	// We don't want concurrent calls to NewConnection that would defeat
	// the point of reusing the existing BLE device. Note that this is not
	// an issue on MacOS, but multiple calls to newDevice() on Linux leads to failures.
	mu.Lock()
	defer mu.Unlock()

	if device != nil {
		log.Debug("Reusing existing BLE device")
	} else {
		log.Debug("Creating new BLE device")
		device, err = newDevice()
		if err != nil {
			return nil, fmt.Errorf("failed to find a BLE device: %s", err)
		}
		ble.SetDefaultDevice(device)
	}

	vinBytes := []byte(vin)
	digest := sha1.Sum(vinBytes)

	localName := fmt.Sprintf("S%02xC", digest[:8])
	log.Debug("Searching for BLE beacon %s...", localName)
	filter := func(adv ble.Advertisement) bool {
		if !adv.Connectable() || adv.LocalName() != localName {
			return false
		}
		return true
	}

	log.Debug("Connecting to BLE beacon...")
	client, err := ble.Connect(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to find BLE beacon for %s (%s): %s", vin, localName, err)
	}

	services, err := client.DiscoverServices([]ble.UUID{vehicleServiceUUID})
	if err != nil {
		return nil, fmt.Errorf("ble: failed to enumerate device services: %s", err)
	}
	if len(services) == 0 {
		return nil, fmt.Errorf("ble: failed to discover service")
	}

	characteristics, err := client.DiscoverCharacteristics([]ble.UUID{toVehicleUUID, fromVehicleUUID}, services[0])
	if err != nil {
		return nil, fmt.Errorf("ble: failed to discover service characteristics: %s", err)
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
			return nil, fmt.Errorf("ble: couldn't fetch descriptors: %s", err)
		}
	}
	if conn.txChar == nil || conn.rxChar == nil {
		return nil, fmt.Errorf("ble: failed to find required characteristics")
	}
	if err := client.Subscribe(conn.rxChar, true, conn.rx); err != nil {
		return nil, fmt.Errorf("ble: failed to subscribe to RX: %s", err)
	}
	log.Info("Connected to vehicle BLE")
	return &conn, nil
}
