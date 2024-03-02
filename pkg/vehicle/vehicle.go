package vehicle

import (
	"context"
	"crypto/ecdh"
	"errors"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/teslamotors/vehicle-command/internal/authentication"
	"github.com/teslamotors/vehicle-command/internal/dispatcher"
	"github.com/teslamotors/vehicle-command/pkg/cache"
	"github.com/teslamotors/vehicle-command/pkg/connector"
	"github.com/teslamotors/vehicle-command/pkg/protocol"

	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/signatures"
	universal "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/universalmessage"
)

var (
	// ErrNoFleetAPIConnection indicates the client attempted to send a command that terminates on
	// Tesla's backend (rather than a vehicle), but the Vehicle Connection does not use connector/inet.
	ErrNoFleetAPIConnection = errors.New("not connected to Fleet API")

	// ErrVehicleStateUnknown indicates the client attempt to determine if a vehicle supported a
	// feature before calling vehicle.GetState.
	ErrVehicleStateUnknown = errors.New("could not determine vehicle state")
)

// sender provides an interface that handles the RoutableMessage protocol layer.
type sender interface {
	// Start causes the sender to listen for messages from the vehicle in a
	// separate go routine. Returns an error if ctx expires before the sender
	// is ready to receive messages.
	Start(ctx context.Context) error

	// Stop the goroutine launched by Start.
	Stop()

	// Send transmits message to the vehicle using the provided authMethod.
	// If err is not nil, the caller must invoke recv.Close() after handling any responses.
	// The client must call StartSessions before calling with auth set to authMethodMAC.
	Send(ctx context.Context, message *universal.RoutableMessage, auth connector.AuthMethod) (recv protocol.Receiver, err error)

	// StartSessions performs handshakes with the vehicle security controller
	// and infotainment to allow subsequent commands to be authenticated.
	StartSessions(ctx context.Context, domains []universal.Domain) error

	Cache() []dispatcher.CacheEntry
	LoadCache(entries []dispatcher.CacheEntry) error

	// Returns the recommended retransmission interval for the Connector
	RetryInterval() time.Duration

	// Sets the maximum allowed clock error.
	SetMaxLatency(time.Duration)
}

// A Vehicle represents a Tesla vehicle.
type Vehicle struct {
	dispatcher sender
	Flags      uint32
	vin        string

	conn       connector.Connector
	authMethod connector.AuthMethod

	keyAvailable bool
}

// NewVehicle creates a new Vehicle. The privateKey and sessionCache may be nil.
func NewVehicle(conn connector.Connector, privateKey authentication.ECDHPrivateKey, sessionCache *cache.SessionCache) (*Vehicle, error) {
	dispatch, err := dispatcher.New(conn, privateKey)
	if err != nil {
		return nil, err
	}
	vin := conn.VIN()
	vehicle := &Vehicle{
		dispatcher:   dispatch,
		vin:          vin,
		conn:         conn,
		authMethod:   conn.PreferredAuthMethod(),
		keyAvailable: privateKey != nil,
	}
	if sessionCache != nil {
		if sessions, ok := sessionCache.GetEntry(vin); ok {
			if err := dispatch.LoadCache(sessions); err != nil {
				return nil, err
			}
		}
	}
	return vehicle, nil
}

// SetMaxLatency sets the threshold used by the client to discard clock-synchronization messages
// from the vehicle that take too long to arrive.
func (v *Vehicle) SetMaxLatency(latency time.Duration) {
	v.dispatcher.SetMaxLatency(latency)
}

func (v *Vehicle) VIN() string {
	return v.vin
}

func (v *Vehicle) PrivateKeyAvailable() bool {
	return v.keyAvailable
}

// Connect opens a connection to the vehicle.
func (v *Vehicle) Connect(ctx context.Context) error {
	return v.dispatcher.Start(ctx)
}

func (v *Vehicle) SessionInfo(ctx context.Context, publicKey *ecdh.PublicKey, domain universal.Domain) (*signatures.SessionInfo, error) {
	request := dispatcher.SessionInfoRequest(domain, publicKey.Bytes())
	recv, err := v.dispatcher.Send(ctx, request, connector.AuthMethodNone)
	if err != nil {
		return nil, err
	}
	select {
	case reply := <-recv.Recv():
		if err := protocol.GetError(reply); err != nil {
			return nil, err
		}
		if infoBytes := reply.GetSessionInfo(); infoBytes != nil {
			var info signatures.SessionInfo
			if err := proto.Unmarshal(infoBytes, &info); err != nil {
				return nil, err
			}
			return &info, nil
		}
		return nil, protocol.ErrBadResponse
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// StartSession performs a handshake with the vehicle that allows the client to begin sending
// authenticated commands. This will fail if the client's public key has not been paired with the
// vehicle. If domains is nil, then the client will establish connections with all supported vehicle
// subsystems. The client may specify a subset of domains if it does not need to connect to all of
// them; for example, a client that only interacts with VCSEC can avoid waking infotainment.
func (v *Vehicle) StartSession(ctx context.Context, domains []universal.Domain) error {
	for {
		err := v.dispatcher.StartSessions(ctx, domains)
		if err == nil {
			return nil
		}

		if !protocol.ShouldRetry(err) {
			return err
		}

		select {
		case <-time.After(v.dispatcher.RetryInterval()):
			continue
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Disconnect closes the connection to v.
// Calling this method invokes the underlying [connector.Connector.Close] method. The
// [connector.Connector] interface definition requires that multiple calls to Close() are safe, and so
// it is safe to defer both this method and the Connector's Close() method; however, Disconnect must
// be invoked first.
func (v *Vehicle) Disconnect() {
	v.dispatcher.Stop()
	if v.conn != nil {
		v.conn.Close()
	}
}

func (v *Vehicle) getReceiver(ctx context.Context, domain universal.Domain, payload []byte, auth connector.AuthMethod) (protocol.Receiver, error) {
	message := universal.RoutableMessage{
		ToDestination: &universal.Destination{
			SubDestination: &universal.Destination_Domain{
				Domain: domain,
			},
		},
		Payload: &universal.RoutableMessage_ProtobufMessageAsBytes{
			ProtobufMessageAsBytes: payload,
		},
		Flags: v.Flags,
	}

	pendingResponse, err := v.dispatcher.Send(ctx, &message, auth)
	if err != nil {
		return nil, err
	}
	return pendingResponse, nil
}

func (v *Vehicle) trySend(ctx context.Context, domain universal.Domain, payload []byte, auth connector.AuthMethod) ([]byte, error) {
	recv, err := v.getReceiver(ctx, domain, payload, auth)
	if err != nil {
		return nil, err
	}
	defer recv.Close()

	select {
	case response := <-recv.Recv():
		return response.GetProtobufMessageAsBytes(), protocol.GetError(response)
	case <-ctx.Done():
		return nil, &protocol.CommandError{Err: ctx.Err(), PossibleSuccess: true, PossibleTemporary: true}
	}
}

// SendMessage sends a routable message to the vehicle.
//
// This interface is intended to be used when proxying commands that were authorized by a different
// entity, notably when using cardless key pairing over BLE. In most cases, you'll want to use Send
// instead, which automatically resynchronises session state and tries again when encountering
// certain types of errors.
//
// The SendMessage method only retries on errors for which retransmission of the same message
// (without modifying anti-replay counters, etc.) is safe and might resolve a transient error.
func (v *Vehicle) SendMessage(ctx context.Context, message *universal.RoutableMessage) (protocol.Receiver, error) {
	return v.dispatcher.Send(ctx, message, connector.AuthMethodNone)
}

// Send a payload to a Vehicle. This is a low-level method that most clients will not need.
//
// The method retries until vehicle responds with a terminal result (success or non-transient
// failure) or the provided context expires.
//
// The domain controls what vehicle subsystem receives the message, and auth controls how the
// message is authenticated (if it all).
func (v *Vehicle) Send(ctx context.Context, domain universal.Domain, payload []byte, auth connector.AuthMethod) ([]byte, error) {
	payloadCopy := make([]byte, len(payload))
	copy(payloadCopy, payload)
	for {
		response, err := v.trySend(ctx, domain, payloadCopy, auth)

		if err == nil {
			return response, nil
		}

		if !protocol.ShouldRetry(err) {
			return nil, err
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(v.dispatcher.RetryInterval()):
			continue
		}
	}
}

func (v *Vehicle) Wakeup(ctx context.Context) error {
	if oapi, ok := v.conn.(connector.FleetAPIConnector); ok {
		return oapi.Wakeup(ctx)
	} else {
		return v.wakeupRKE(ctx)
	}
}

func (v *Vehicle) UpdateCachedSessions(c *cache.SessionCache) error {
	return c.Update(v.vin, v.dispatcher.Cache())
}

func (v *Vehicle) LoadCachedSessions(c *cache.SessionCache) error {
	if data, ok := c.GetEntry(v.vin); ok {
		return v.dispatcher.LoadCache(data)
	}
	return errors.New("VIN not in cache")
}
