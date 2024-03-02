package dispatcher

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/teslamotors/vehicle-command/internal/authentication"
	"github.com/teslamotors/vehicle-command/internal/log"
	"github.com/teslamotors/vehicle-command/pkg/connector"
	"github.com/teslamotors/vehicle-command/pkg/protocol"

	"google.golang.org/protobuf/proto"

	universal "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/universalmessage"
)

// Dispatcher objects send (encrypted) messages to a vehicle and route incoming messages to the
// appropriate receiver object.
type Dispatcher struct {
	conn       connector.Connector
	privateKey authentication.ECDHPrivateKey
	address    []byte

	latencyLock sync.Mutex
	maxLatency  time.Duration

	doneLock  sync.Mutex
	terminate chan struct{}
	done      chan bool

	sessionLock sync.Mutex
	sessions    map[universal.Domain]*session

	handlerLock sync.Mutex
	handlers    map[receiverKey]*receiver
}

// New creates a Dispatcher from a Connector.
func New(conn connector.Connector, privateKey authentication.ECDHPrivateKey) (*Dispatcher, error) {
	dispatcher := Dispatcher{
		conn:       conn,
		maxLatency: conn.AllowedLatency(),
		address:    make([]byte, addressLength),
		sessions:   make(map[universal.Domain]*session),
		handlers:   make(map[receiverKey]*receiver),
		privateKey: privateKey,
		done:       make(chan bool),
	}
	if _, err := rand.Read(dispatcher.address); err != nil {
		return nil, err
	}
	return &dispatcher, nil
}

func (d *Dispatcher) SetMaxLatency(latency time.Duration) {
	if latency > 0 {
		d.latencyLock.Lock()
		d.maxLatency = latency
		d.latencyLock.Unlock()
	}
}

// RetryInterval fetches the transport-layer dependent recommended delay between retry attempts.
func (d *Dispatcher) RetryInterval() time.Duration {
	return d.conn.RetryInterval()
}

// StartSession sends a blocking request start an authenticated session with a universal.Domain.
func (d *Dispatcher) StartSession(ctx context.Context, domain universal.Domain) error {
	var err error
	var sessionReady bool
	d.sessionLock.Lock()
	s, ok := d.sessions[domain]
	if !ok {
		d.sessions[domain], err = NewSession(d.privateKey, d.conn.VIN())
		s = d.sessions[domain]
	} else if s != nil && s.ctx != nil {
		log.Info("Session for %s loaded from cache", domain)
		sessionReady = true
	}
	d.sessionLock.Unlock()
	if err != nil || sessionReady {
		return err
	}
	for {
		recv, err := d.RequestSessionInfo(ctx, domain)
		if err != nil {
			return err
		}
		defer recv.Close()
		select {
		case reply := <-recv.Recv():
			if err = protocol.GetError(reply); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		case <-s.readySignal:
			return nil
		}
		select {
		case <-time.After(d.conn.RetryInterval()):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// StartSessions starts sessions with the provided vehicle domains (or all supported domains, if
// domains is nil).
//
// If multiple connections fail, only returns the first error.
func (d *Dispatcher) StartSessions(ctx context.Context, domains []universal.Domain) error {
	aggregateContext, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make(chan error)
	if domains == nil {
		domains = []universal.Domain{
			universal.Domain_DOMAIN_VEHICLE_SECURITY,
			universal.Domain_DOMAIN_INFOTAINMENT,
		}
	}
	for _, domain := range domains {
		go func(dom universal.Domain) {
			results <- d.StartSession(aggregateContext, dom)
		}(domain)
	}
	var err error
	for i := 0; i < len(domains); i++ {
		err = <-results
		// The aggregateContext is canceled if one of the handshakes fails. We don't want to return
		// the Canceled error if ErrProtocolNotSupported is present.
		if err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
	}
	return err
}

func (d *Dispatcher) createHandler(key *receiverKey) *receiver {
	d.handlerLock.Lock()
	defer d.handlerLock.Unlock()

	now := time.Now()
	recv := &receiver{
		key:           key,
		ch:            make(chan *universal.RoutableMessage, receiverBufferSize),
		dispatcher:    d,
		requestSentAt: now,
		lastActive:    now,
	}

	d.handlers[*key] = recv
	return recv
}

func (d *Dispatcher) closeHandler(recv *receiver) {
	d.handlerLock.Lock()
	delete(d.handlers, *recv.key)
	d.handlerLock.Unlock()
}

func (d *Dispatcher) checkForSessionUpdate(message *universal.RoutableMessage, handler *receiver) {
	domain := handler.key.domain
	sessionInfo := message.GetSessionInfo()
	if sessionInfo == nil {
		return
	}

	if d.privateKey == nil {
		log.Warning("[%02x] Discarding session info because client does not have a private key", message.GetRequestUuid())
		return
	}

	d.latencyLock.Lock()
	maxLatency := d.maxLatency
	d.latencyLock.Unlock()

	if handler.expired(maxLatency) {
		log.Warning("[%02x] Discarding session info because it was received more than %s after request", message.GetRequestUuid(), maxLatency)
		return
	}

	tag := message.GetSignatureData().GetSessionInfoTag().GetTag()
	if tag == nil {
		log.Warning("[%02x] Discarding unauthenticated session info", message.GetRequestUuid())
	}
	var err error

	d.sessionLock.Lock()
	defer d.sessionLock.Unlock()

	session, ok := d.sessions[domain]
	if !ok {
		log.Error("[%02x] Dropping session from unregistered domain %s", message.GetRequestUuid(), domain)
		return
	}

	if err = session.ProcessHello(message.GetRequestUuid(), sessionInfo, tag); err != nil {
		log.Warning("[%02x] Session info error: %s", message.GetRequestUuid(), err)
		return
	}
	log.Info("[%02x] Updated session info for %s", message.GetRequestUuid(), domain)
}

func (d *Dispatcher) process(message *universal.RoutableMessage) {
	var key receiverKey

	if message.GetFromDestination() == nil {
		log.Warning("[xxx] Dropping message with missing source")
		return
	}
	key.domain = message.GetFromDestination().GetDomain()

	requestUUID := message.GetRequestUuid()
	if len(requestUUID) != uuidLength && len(requestUUID) != 0 {
		log.Warning("[xxx] Dropping message with invalid request UUID length")
		return
	}
	if key.domain != universal.Domain_DOMAIN_VEHICLE_SECURITY {
		copy(key.uuid[:], requestUUID)
	}

	destination := message.GetToDestination()
	if destination == nil {
		log.Warning("[%02x] Dropping message with missing destination", message.GetRequestUuid())
		return
	}

	switch d := destination.SubDestination.(type) {
	case *universal.Destination_Domain:
		log.Debug("[%02x] Dropping message to %s", message.GetRequestUuid(), d.Domain)
		return
	case *universal.Destination_RoutingAddress:
		// Continue
	default:
		log.Debug("[%02x] Dropping message with unrecognized destination type", message.GetRequestUuid())
		return
	}

	addr := destination.GetRoutingAddress()
	if len(addr) != addressLength {
		log.Warning("[%02x] Dropping message with invalid address length", message.GetRequestUuid())
		return
	}
	copy(key.address[:], addr)

	d.handlerLock.Lock()
	handler, ok := d.handlers[key]
	d.handlerLock.Unlock()
	if !ok {
		log.Warning("[%02x] Dropping message without registered handler %s", requestUUID, key.String())
		return
	}

	// Vehicles may proactively include session info if they believe there may
	// have been a desync. This typically accompanies an error message, and so
	// the reply still needs to be passed down to the handler after updating
	// session info.
	d.checkForSessionUpdate(message, handler)

	select {
	case handler.ch <- message:
	default:
		log.Error("[%02x] Dropping response to command because response handler queue is full", requestUUID)
	}
}

// Start runs d's Listen method in a new goroutine. Returns an error if d does
// not signal it's ready before ctx expires.
func (d *Dispatcher) Start(ctx context.Context) error {
	ready := make(chan struct{})
	go d.listen(ready)
	select {
	case <-ready:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Listen for incoming commands and dispatch them to registered receivers.
func (d *Dispatcher) listen(ready chan<- struct{}) {
	log.Info("Starting dispatcher service...")
	d.doneLock.Lock()
	if d.terminate == nil {
		d.terminate = make(chan struct{})
	} else {
		d.doneLock.Unlock()
		return
	}
	terminate := d.terminate
	d.doneLock.Unlock()
	listening := make(chan struct{}, 2)
	listening <- struct{}{}
	defer func() {
		d.done <- true
	}()
	for {
		select {
		case messageBytes, open := <-d.conn.Receive():
			if !open {
				return
			}
			message := new(universal.RoutableMessage)
			if err := proto.Unmarshal(messageBytes, message); err != nil {
				log.Warning("Dropping unparseable message: %s", err)
				continue
			}
			d.process(message)
		case <-terminate:
			return
		case <-listening:
			close(ready)
		}
	}
}

// Stop signals any goroutine running Listen to exit.
func (d *Dispatcher) Stop() {
	d.doneLock.Lock()
	defer d.doneLock.Unlock()
	if d.terminate != nil {
		close(d.terminate)
		d.terminate = nil
		<-d.done
	}
}

// Send a message to a vehicle.
func (d *Dispatcher) Send(ctx context.Context, message *universal.RoutableMessage, auth connector.AuthMethod) (protocol.Receiver, error) {
	d.doneLock.Lock()
	listening := d.terminate != nil
	d.doneLock.Unlock()
	if !listening {
		return nil, protocol.ErrNotConnected
	}
	var key receiverKey
	key.domain = message.GetToDestination().GetDomain()
	if key.domain == universal.Domain_DOMAIN_BROADCAST {
		return nil, protocol.NewError("cannot send message without a destination domain", false, false)
	}

	addr := make([]byte, addressLength)
	// Message UUIDs are only used for debugging message logs and are not
	// copied into the receiverKey used to match responses to requests.
	uuid := make([]byte, uuidLength)
	if _, err := rand.Read(uuid); err != nil {
		return nil, err
	}

	if key.domain == universal.Domain_DOMAIN_VEHICLE_SECURITY {
		if _, err := rand.Read(addr); err != nil {
			return nil, err
		}
	} else {
		copy(addr, d.address)
		copy(key.uuid[:], uuid)
	}

	copy(key.address[:], addr)
	message.Uuid = uuid
	message.FromDestination = &universal.Destination{
		SubDestination: &universal.Destination_RoutingAddress{RoutingAddress: addr},
	}

	if auth != connector.AuthMethodNone {
		d.sessionLock.Lock()
		session, ok := d.sessions[message.GetToDestination().GetDomain()]
		if ok {
			session.lock.Lock()
			ok = session.ready
			session.lock.Unlock()
		}
		d.sessionLock.Unlock()
		if !ok {
			log.Warning("No session available for %s", message.GetToDestination().GetDomain())
			return nil, protocol.ErrNoSession
		}
		if err := session.Authorize(ctx, message, auth); err != nil {
			return nil, err
		}
	}

	resp := d.createHandler(&key)
	encodedMessage, err := proto.Marshal(message)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			resp.Close()
		}
	}()

	for {
		err = d.conn.Send(ctx, encodedMessage)
		if err == nil {
			return resp, nil
		}
		if !protocol.ShouldRetry(err) {
			log.Warning("[%02x] Terminal transmission error: %s", message.GetUuid(), err)
			return nil, err
		}
		log.Debug("[%02x] Retrying transmission after error: %s", message.GetUuid(), err)
		select {
		case <-ctx.Done():
			return nil, &protocol.CommandError{Err: ctx.Err(), PossibleSuccess: false, PossibleTemporary: true}
		case <-time.After(d.conn.RetryInterval()):
			continue
		}
	}
}

// SessionInfoRequest returns a RoutableMesasge that initiates a handshake with a vehicle Domain.
func SessionInfoRequest(domain universal.Domain, publicBytes []byte) *universal.RoutableMessage {
	request := universal.RoutableMessage{
		ToDestination: &universal.Destination{
			SubDestination: &universal.Destination_Domain{
				Domain: domain,
			},
		},
		Payload: &universal.RoutableMessage_SessionInfoRequest{
			SessionInfoRequest: &universal.SessionInfoRequest{
				PublicKey: publicBytes,
			},
		},
	}
	return &request
}

// RequestSessionInfo sends a handshake request and returns a protocol.Receiver for receiving the
// response.
func (d *Dispatcher) RequestSessionInfo(ctx context.Context, domain universal.Domain) (protocol.Receiver, error) {
	log.Info("Requesting session info from %s", domain)
	if d.privateKey == nil {
		return nil, protocol.ErrRequiresKey
	}
	return d.Send(ctx, SessionInfoRequest(domain, d.privateKey.PublicBytes()), connector.AuthMethodNone)
}

// Cache returns a list of CacheEntry objects that contain session state for the authenticated
// connections to a vehicle's Domains.
func (d *Dispatcher) Cache() []CacheEntry {
	d.sessionLock.Lock()
	defer d.sessionLock.Unlock()
	var entries []CacheEntry
	for domain, session := range d.sessions {
		if session == nil {
			continue
		}
		encodedInfo := session.export()
		if encodedInfo == nil {
			continue
		}
		entry := CacheEntry{
			CreatedAt:   time.Now(),
			Domain:      int(domain),
			SessionInfo: encodedInfo,
		}
		entries = append(entries, entry)
	}
	return entries
}

// LoadCache initializes or overwrites d's sessions. This allows resuming a session with a vehicle
// without requiring a round trip.
func (d *Dispatcher) LoadCache(entries []CacheEntry) error {
	sessions := make(map[universal.Domain]*session)
	for _, entry := range entries {
		s, err := NewSession(d.privateKey, d.conn.VIN())
		close(s.readySignal)
		s.ready = true
		if err != nil {
			return fmt.Errorf("invalid cache: %s", err)
		}
		s.ctx, err = authentication.ImportSessionInfo(d.privateKey, []byte(d.conn.VIN()), entry.SessionInfo, entry.CreatedAt)
		if err != nil {
			return fmt.Errorf("invalid cache: %s", err)
		}
		sessions[universal.Domain(entry.Domain)] = s
	}

	d.sessionLock.Lock()
	defer d.sessionLock.Unlock()
	d.sessions = sessions
	return nil
}
