package ble

import (
	"context"
	"crypto/sha1"
	"fmt"
	"sync"
	"time"

	"github.com/teslamotors/vehicle-command/internal/log"
	"github.com/teslamotors/vehicle-command/pkg/connector"
	"github.com/teslamotors/vehicle-command/pkg/protocol"
)

var (
	ErrMaxConnectionsExceeded = protocol.NewError("the vehicle is already connected to the maximum number of BLE devices", false, false)
)

const (
	defaultMTU        = 23
	maxBLEMTUSize     = 512 + 3
	maxBLEMessageSize = 1024

	rxTimeout  = time.Second     // Timeout interval between receiving chunks of a mesasge
	maxLatency = 4 * time.Second // Max allowed error when syncing vehicle clock
)

const (
	vehicleServiceUUID = "00000211-b2d1-43f0-9b88-960cebf8b91e"
	toVehicleUUID      = "00000212-b2d1-43f0-9b88-960cebf8b91e"
	fromVehicleUUID    = "00000213-b2d1-43f0-9b88-960cebf8b91e"
)

func VehicleLocalName(vin string) string {
	vinBytes := []byte(vin)
	digest := sha1.Sum(vinBytes)
	return fmt.Sprintf("S%02xC", digest[:8])
}

type Connection struct {
	vin    string
	inbox  chan []byte
	device Device
	writer Writer

	blockLength int
	inputBuffer []byte
	lastRx      time.Time
	lock        sync.Mutex
}

func ScanVehicleBeacon(ctx context.Context, vin string, adapter Adapter) (*Beacon, error) {
	return adapter.ScanBeacon(ctx, VehicleLocalName(vin))
}

func NewConnection(ctx context.Context, vin string, adapter Adapter) (*Connection, error) {
	beacon, err := adapter.ScanBeacon(ctx, VehicleLocalName(vin))
	if err != nil {
		return nil, err
	}
	return NewConnectionFromBeacon(ctx, vin, beacon, adapter)
}

func NewConnectionFromBeacon(ctx context.Context, vin string, beacon *Beacon, adapter Adapter) (*Connection, error) {
	var lastError error

	if beacon.LocalName != VehicleLocalName(vin) {
		return nil, fmt.Errorf("ble: beacon with unexpected local name: '%s'", beacon.LocalName)
	}

	if !beacon.Connectable {
		return nil, ErrMaxConnectionsExceeded
	}

	for {
		conn, err := tryToConnect(ctx, vin, beacon, adapter)
		if err == nil {
			return conn, nil
		}

		log.Warning("BLE connection attempt failed: %+v", err)
		if err := ctx.Err(); err != nil {
			if lastError != nil {
				return nil, lastError
			}
			return nil, err
		}
		lastError = err
	}
}

func tryToConnect(ctx context.Context, vin string, beacon *Beacon, adapter Adapter) (*Connection, error) {
	device, err := adapter.Connect(ctx, beacon)
	if err != nil {
		return nil, err
	}

	service, err := device.Service(ctx, vehicleServiceUUID)
	if err != nil {
		return nil, err
	}

	writer, err := service.Tx(toVehicleUUID)
	if err != nil {
		return nil, err
	}

	txMtu, err := writer.MTU(maxBLEMTUSize)
	if err != nil {
		txMtu = defaultMTU - 3 // Fallback to default MTU size
	} else {
		txMtu = min(txMtu, maxBLEMessageSize) - 3 // 3 bytes for header
	}

	conn := &Connection{
		vin:    vin,
		inbox:  make(chan []byte, 5),
		device: device,
		writer: writer,

		blockLength: txMtu,
	}

	err = service.Rx(fromVehicleUUID, conn.rx)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func (c *Connection) Receive() <-chan []byte {
	return c.inbox
}

func (c *Connection) Send(ctx context.Context, buffer []byte) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	var out []byte
	log.Debug("TX: %02x", buffer)
	out = append(out, uint8(len(buffer)>>8), uint8(len(buffer)))
	out = append(out, buffer...)
	blockLength := c.blockLength
	for len(out) > 0 {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if blockLength > len(out) {
			blockLength = len(out)
		}

		n, err := c.writer.Write(out[:blockLength])
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

func (c *Connection) Close() {
	if err := c.device.Close(); err != nil {
		log.Warning("ble: failed to close device: %s", err)
	}
}

func (c *Connection) PreferredAuthMethod() connector.AuthMethod {
	return connector.AuthMethodGCM
}

func (c *Connection) RetryInterval() time.Duration {
	return time.Second
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
