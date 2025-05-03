package conn

import (
	"context"
	"github.com/teslamotors/vehicle-command/internal/log"
	"github.com/teslamotors/vehicle-command/pkg/connector"
	"sync"
	"time"
)

const (
	MaxBLEMessageSize = 1024

	rxTimeout  = time.Second     // Timeout interval between receiving chunks of a message
	maxLatency = 4 * time.Second // Max allowed error when syncing vehicle clock

	VehicleServiceUUID = "00000211-b2d1-43f0-9b88-960cebf8b91e"
	ToVehicleUUID      = "00000212-b2d1-43f0-9b88-960cebf8b91e"
	FromVehicleUUID    = "00000213-b2d1-43f0-9b88-960cebf8b91e"
)

type Writer interface {
	WriteCharacteristic(bytes []byte, length int) error
	Close()
}

func NewConnection(vin string, blockLength int, writer Writer) *Connection {
	return &Connection{
		vin:         vin,
		inbox:       make(chan []byte, 5),
		blockLength: blockLength,
		writer:      writer,
	}
}

type Connection struct {
	vin         string
	inbox       chan []byte
	blockLength int
	inputBuffer []byte

	lastRx time.Time
	lock   sync.Mutex

	writer Writer
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
		if msgLength > MaxBLEMessageSize {
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
	c.writer.Close()
}

func (c *Connection) AllowedLatency() time.Duration {
	return maxLatency
}

func (c *Connection) Rx(p []byte) {
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

		if err := c.writer.WriteCharacteristic(out[:blockLength], blockLength); err != nil {
			return err
		}
		out = out[blockLength:]
	}
	return nil
}

func (c *Connection) VIN() string {
	return c.vin
}

func (c *Connection) SetBlockLength(blockLength int) {
	c.blockLength = blockLength
}
