package connector

import (
	"context"
	"time"
)

// AuthMethod enumerates the different mechanisms vehicles use to authenticate clients.
type AuthMethod int32

const (
	// AuthMethodNone commnds are unauthenticated. They are typically used for handshake messages.
	AuthMethodNone AuthMethod = iota

	// AuthMethodGCM commands are authenticated and encrypted with AES-GCM-ECDH.
	AuthMethodGCM

	// AuthMethodHMAC commands are authenticated with HMAC-SHA256-ECDH.
	AuthMethodHMAC
)

// BufferSize is the number of inbound messages that can be queued.
const BufferSize = 5

// MaxResponseLength caps the maximum byte-length of responses that connectors must support.
const MaxResponseLength = 100000

// Connector send and receives raw datagrams ([]byte) from a vehicle.
type Connector interface {
	// Receive returns a read-only channel used to receive datagrams sent by the vehicle.
	//
	// Implementations must be thread safe.
	Receive() <-chan []byte

	// Send sends a buffer to the vehicle.
	//
	// Depending on the error, the vehicle may have received and even acted on the message. For some
	// errors, such as network timeouts, the client will not be able to determine if this is the
	// case. If the returned error implements the vehicle.Error interface, then the client may be
	// able to determine if the message was received by using the appropriate methods.
	//
	// Implementations must be thread safe.
	Send(ctx context.Context, buffer []byte) error

	// VIN returns the vehicle identification number of the connected vehicle.
	VIN() string

	// Close terminates the connection a vehicle.
	//
	// Repeated calls to Close() must be idempotent, but the behavior of the interface is otherwise
	// undefined after calling this method.
	Close()

	// PreferredAuthMethod returns the AuthMethod that a Dispatcher should use with this connection.
	// An HTTP-based Connector requires AuthMethodHMAC.
	PreferredAuthMethod() AuthMethod

	// RetryInterval returns the recommended wait time between transmission attempts.
	RetryInterval() time.Duration

	// AllowedLatency returns the maximum permitted delay between sending a request and receiving a
	// response with an updated vehicle clock.
	AllowedLatency() time.Duration
}

// FleetAPIConnector is a superset of Connector (which sends datagrams to vehicles) that also allows
// sending commands to Fleet API.
type FleetAPIConnector interface {
	Connector
	SendFleetAPICommand(ctx context.Context, endpoint string, command interface{}) ([]byte, error)
	Wakeup(ctx context.Context) error
}
