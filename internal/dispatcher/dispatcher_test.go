package dispatcher

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/teslamotors/vehicle-command/pkg/connector"
	"github.com/teslamotors/vehicle-command/pkg/protocol"

	"google.golang.org/protobuf/proto"

	"github.com/teslamotors/vehicle-command/internal/authentication"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/signatures"
	universal "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/universalmessage"
)

var errOutboxFull = errors.New("dispatcher: outbox full")
var errDropMessage = errors.New("dispatcher: simulated dropped message")
var errTimeout = errors.New("dispatcher: simulated timeout")
var testPayload = []byte("ack")
var quiescentDelay = 250 * time.Millisecond

const testDomain = universal.Domain_DOMAIN_INFOTAINMENT

func checkFault(t *testing.T, message *universal.RoutableMessage, fault universal.MessageFault_E) {
	t.Helper()
	observedFault := message.GetSignedMessageStatus().GetSignedMessageFault()
	if observedFault != fault {
		t.Errorf("Expected fault %s but got %s", fault, observedFault)
	}
}

func testUUID() []byte {
	id := make([]byte, uuidLength)
	for i := 0; i < uuidLength; i++ {
		id[i] = uint8(i)
	}
	return id
}

func populateReplyMetadata(rsp protocol.Receiver, reply *universal.RoutableMessage) {
	r := rsp.(*receiver)
	address := make([]byte, len(r.key.address))
	copy(address, r.key.address[:])
	reply.ToDestination = &universal.Destination{
		SubDestination: &universal.Destination_RoutingAddress{
			RoutingAddress: address,
		},
	}
	reply.FromDestination = &universal.Destination{
		SubDestination: &universal.Destination_Domain{Domain: r.key.domain},
	}
	reqUUID := make([]byte, len(r.key.uuid))
	copy(reqUUID, r.key.uuid[:])
	reply.RequestUuid = reqUUID
	reply.Uuid = testUUID()
}

func replyWithPayload(rsp protocol.Receiver, payload []byte) *universal.RoutableMessage {
	var reply universal.RoutableMessage
	populateReplyMetadata(rsp, &reply)
	data := []byte{}
	data = append(data, payload...)
	reply.Payload = &universal.RoutableMessage_ProtobufMessageAsBytes{
		ProtobufMessageAsBytes: data,
	}
	return &reply
}

func testCommand() *universal.RoutableMessage {
	return &universal.RoutableMessage{
		ToDestination: &universal.Destination{
			SubDestination: &universal.Destination_Domain{Domain: testDomain},
		},
		Payload: &universal.RoutableMessage_ProtobufMessageAsBytes{ProtobufMessageAsBytes: []byte("hello")},
	}
}

type dummyConnector struct {
	inbox       []*universal.RoutableMessage
	callback    func(*dummyConnector, *universal.RoutableMessage) ([]byte, bool)
	outbox      chan []byte
	replies     chan *universal.RoutableMessage
	lock        sync.Mutex
	errorQueue  []error
	keyLock     sync.Mutex
	keys        map[universal.Domain]authentication.ECDHPrivateKey
	dropReplies bool
	AckRequests bool
}

func newDummyConnector(t *testing.T) *dummyConnector {
	t.Helper()
	conn := dummyConnector{
		callback:    handleSessionInfoRequests,
		outbox:      make(chan []byte, 50),
		replies:     make(chan *universal.RoutableMessage, 50),
		keys:        make(map[universal.Domain]authentication.ECDHPrivateKey),
		dropReplies: false,
		AckRequests: true,
	}
	return &conn
}

func (d *dummyConnector) PreferredAuthMethod() connector.AuthMethod {
	return connector.AuthMethodHMAC
}

func (d *dummyConnector) Sleep() {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.dropReplies = true
}

func (d *dummyConnector) Wake() {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.dropReplies = false
}

func (d *dummyConnector) domainKey(domain universal.Domain) authentication.ECDHPrivateKey {
	d.keyLock.Lock()
	defer d.keyLock.Unlock()
	if key, ok := d.keys[domain]; ok {
		return key
	}
	var err error
	d.keys[domain], err = authentication.NewECDHPrivateKey(rand.Reader)
	if err != nil {
		panic(err)
	}
	return d.keys[domain]
}

func (d *dummyConnector) AllowedLatency() time.Duration {
	return time.Second
}

func (d *dummyConnector) RetryInterval() time.Duration {
	return time.Millisecond
}

func (d *dummyConnector) EnqueueReply(t *testing.T, response []byte) {
	respCopy := make([]byte, len(response))
	copy(respCopy, response)
	t.Helper()
	d.lock.Lock()
	defer d.lock.Unlock()
	if d.dropReplies {
		return
	}
	select {
	case d.outbox <- respCopy:
	default:
		t.Fatalf("Couldn't queue message")
	}
}

func (d *dummyConnector) VIN() string {
	return "0123456789ABCDEFG"
}

func (d *dummyConnector) Receive() <-chan []byte {
	return d.outbox
}

func (d *dummyConnector) Close() {
	d.lock.Lock()
	d.dropReplies = true
	close(d.outbox)
	d.lock.Unlock()
}

func (d *dummyConnector) SessionInfoReply(rsp protocol.Receiver, publicKeyBytes []byte) *universal.RoutableMessage {
	r := rsp.(*receiver)
	domain := r.key.domain
	verifier, err := authentication.NewVerifier(d.domainKey(domain), []byte(d.VIN()), domain, publicKeyBytes)
	if err != nil {
		panic(err)
	}

	reply := &universal.RoutableMessage{}
	if err := verifier.SetSessionInfo(r.key.uuid[:], reply); err != nil {
		panic(err)
	}
	populateReplyMetadata(rsp, reply)
	return reply
}

func initReply(message *universal.RoutableMessage) *universal.RoutableMessage {
	var reply universal.RoutableMessage
	reply.ToDestination = message.GetFromDestination()
	reply.FromDestination = message.GetToDestination()
	reply.RequestUuid = append([]byte{}, message.GetUuid()...)
	reply.Uuid = testUUID()
	return &reply
}

func handleSessionInfoRequests(d *dummyConnector, message *universal.RoutableMessage) ([]byte, bool) {
	req := message.GetSessionInfoRequest()
	if req == nil {
		return nil, false
	}

	domain := message.GetToDestination().GetDomain()
	verifier, err := authentication.NewVerifier(d.domainKey(domain), []byte(d.VIN()), domain, req.GetPublicKey())
	if err != nil {
		panic(err)
	}

	reply := initReply(message)
	if err := verifier.SetSessionInfo(message.GetUuid(), reply); err != nil {
		panic(err)
	}

	encoded, err := proto.Marshal(reply)
	if err != nil {
		panic(err)
	}

	return encoded, true
}

func (d *dummyConnector) handleAsync(message *universal.RoutableMessage) {
	d.lock.Lock()
	defer d.lock.Unlock()
	d.inbox = append(d.inbox, message)
	if d.dropReplies || d.callback == nil {
		return
	}
	if responseBytes, shouldSend := d.callback(d, message); shouldSend {
		select {
		case d.outbox <- responseBytes:
			return
		default:
			panic(errOutboxFull)
		}
	}
}

func (d *dummyConnector) EnqueueSendError(err error) {
	d.lock.Lock()
	d.errorQueue = append(d.errorQueue, err)
	d.lock.Unlock()
}

func (d *dummyConnector) Send(ctx context.Context, buffer []byte) error {
	var message universal.RoutableMessage
	if !d.AckRequests {
		return errTimeout
	}
	if len(d.errorQueue) > 0 {
		d.lock.Lock()
		err := d.errorQueue[0]
		d.errorQueue = d.errorQueue[1:]
		d.lock.Unlock()
		if err == errDropMessage {
			return nil
		} else if err != nil {
			return err
		}
	}
	if err := proto.Unmarshal(buffer, &message); err != nil {
		return err
	}
	go d.handleAsync(&message)
	return nil
}

func TestSendWithoutSession(t *testing.T) {
	conn := newDummyConnector(t)
	defer conn.Close()

	key, err := authentication.NewECDHPrivateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Couldn't create private key: %s", err)
	}
	dispatcher, err := New(conn, key)
	if err != nil {
		t.Fatalf("Couldn't initialize dispatcher: %s", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := dispatcher.Start(ctx); err != nil {
		t.Fatal(err)
	}

	if _, err = dispatcher.Send(ctx, testCommand(), connector.AuthMethodHMAC); err != protocol.ErrNoSession {
		t.Errorf("Expected ErrNoSession but got %s", err)
	}

	if _, err = dispatcher.Send(ctx, testCommand(), connector.AuthMethodNone); err != nil {
		t.Errorf("Error sending unauthenticated message: %s", err)
	}
}

// getTestSetup creates and returns a Dispatcher and associated dummyConnector. This function launches the dispatcher's Listen goroutine, and the caller must Close() the returned dummyConnector.
func getTestSetup(t *testing.T) (*Dispatcher, *dummyConnector) {
	t.Helper()
	conn := newDummyConnector(t)

	key, err := authentication.NewECDHPrivateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Couldn't create private key: %s", err)
	}
	dispatcher, err := New(conn, key)
	if err != nil {
		t.Fatalf("Couldn't initialize dispatcher: %s", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), quiescentDelay)
	defer cancel()
	if err := dispatcher.Start(ctx); err != nil {
		t.Fatal(err)
	}
	err = dispatcher.StartSession(ctx, testDomain)
	if err != nil {
		t.Fatalf("Couldn't start session: %s", err)
	}
	return dispatcher, conn
}

func TestStartSession(t *testing.T) {
	dispatcher, conn := getTestSetup(t)
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), quiescentDelay)
	defer cancel()

	rsp, err := dispatcher.Send(ctx, testCommand(), connector.AuthMethodHMAC)
	if err != nil {
		t.Fatalf("Error getting response: %s", err)
	}

	conn.EnqueueReply(t, encodeRoutableMessage(t, replyWithPayload(rsp, []byte("hello world"))))

	select {
	case message := <-rsp.Recv():
		checkFault(t, message, universal.MessageFault_E_MESSAGEFAULT_ERROR_NONE)
	case <-ctx.Done():
		t.Errorf("Timed out waiting for response")
	}
}

func TestTimeout(t *testing.T) {
	dispatcher, conn := getTestSetup(t)
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), quiescentDelay)
	defer cancel()

	rsp, err := dispatcher.Send(ctx, testCommand(), connector.AuthMethodHMAC)
	if err != nil {
		t.Fatalf("Error getting response: %s", err)
	}

	select {
	case message := <-rsp.Recv():
		t.Errorf("Received response when expecting a timeout: %+v", message)
	case <-ctx.Done():
		if ctx.Err() != context.DeadlineExceeded {
			t.Errorf("Expected deadline exceeded error but got %s", ctx.Err())
		}
	}
}

func encodeRoutableMessage(t *testing.T, message *universal.RoutableMessage) []byte {
	t.Helper()
	if encoded, err := proto.Marshal(message); err != nil {
		t.Fatalf("Error encoding protobuf: %s", err)
	} else {
		return encoded
	}
	return nil
}

func TestInvalidMessages(t *testing.T) {
	dispatcher, conn := getTestSetup(t)
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*quiescentDelay)
	defer cancel()

	rsp, err := dispatcher.Send(ctx, testCommand(), connector.AuthMethodHMAC)
	if err != nil {
		t.Fatalf("Error sending command: %s", err)
	}
	defer rsp.Close()

	// Dump a bunch of invalid replies in the queue and verify that the
	// dispatcher filters all of them out.

	conn.EnqueueReply(t, []byte("I'm not a valid protobuf"))

	missingUUID := replyWithPayload(rsp, []byte("missing uuid"))
	missingUUID.RequestUuid = nil
	conn.EnqueueReply(t, encodeRoutableMessage(t, missingUUID))

	badAddress := replyWithPayload(rsp, []byte("bad destination address"))
	address := badAddress.GetToDestination().GetRoutingAddress()
	address[0] ^= 1
	conn.EnqueueReply(t, encodeRoutableMessage(t, badAddress))

	invalidSource := replyWithPayload(rsp, []byte("invalid domain"))
	invalidSource.FromDestination = &universal.Destination{
		SubDestination: &universal.Destination_Domain{Domain: testDomain + 1},
	}
	conn.EnqueueReply(t, encodeRoutableMessage(t, invalidSource))

	missingDomain := replyWithPayload(rsp, []byte("missing domain"))
	missingDomain.FromDestination = nil
	conn.EnqueueReply(t, encodeRoutableMessage(t, missingDomain))

	missingAddress := replyWithPayload(rsp, []byte("missing destination address"))
	missingAddress.ToDestination = nil
	conn.EnqueueReply(t, encodeRoutableMessage(t, missingAddress))

	missingUUID = replyWithPayload(rsp, []byte("missing request uuid"))
	missingUUID.RequestUuid = nil
	conn.EnqueueReply(t, encodeRoutableMessage(t, missingUUID))

	unknownUUID := replyWithPayload(rsp, []byte("unknown uuid"))
	unknownUUID.RequestUuid[0] ^= 1
	conn.EnqueueReply(t, encodeRoutableMessage(t, unknownUUID))

	// ...and make sure the valid response gets through.
	conn.EnqueueReply(t, encodeRoutableMessage(t, replyWithPayload(rsp, testPayload)))

	time.Sleep(quiescentDelay)

	select {
	case message := <-rsp.Recv():
		payload := message.GetProtobufMessageAsBytes()
		if !bytes.Equal(payload, testPayload) {
			t.Errorf("Unexpected payload: %s", payload)
		}
		// make sure nothing else gets through
		empty := false
		for !empty {
			select {
			case messageB := <-rsp.Recv():
				t.Logf("[%02x] Message: %s", messageB.GetUuid(), messageB.GetProtobufMessageAsBytes())
			default:
				empty = true
			}
		}
	case <-ctx.Done():
		t.Errorf("Unexpected context cancellation: %s", ctx.Err())
	}
}

func TestVehicleDropsReply(t *testing.T) {
	dispatcher, conn := getTestSetup(t)
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), quiescentDelay)
	defer cancel()

	conn.Sleep()
	delete(dispatcher.sessions, testDomain)
	err := dispatcher.StartSession(ctx, testDomain)
	if err != context.DeadlineExceeded {
		t.Errorf("Expected timeout but got error: %s", err)
	}
}

func TestUnsolicitedSessionInfo(t *testing.T) {
	dispatcher, conn := getTestSetup(t)
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), quiescentDelay)
	defer cancel()

	req := testCommand()
	req.ToDestination = &universal.Destination{
		SubDestination: &universal.Destination_Domain{Domain: testDomain + 1},
	}

	// Verify that we can send an unauthorized command to this domain
	rsp, err := dispatcher.Send(ctx, req, connector.AuthMethodNone)
	if err != nil {
		t.Fatalf("Error sending command: %s", err)
	}
	defer rsp.Close()

	// Corrupt authentication tag
	reply := conn.SessionInfoReply(rsp, dispatcher.privateKey.PublicBytes())
	tag := reply.GetSignatureData().GetSessionInfoTag().GetTag()
	if len(tag) == 0 {
		t.Fatal("Reply didn't include a session info HMAC")
	}
	tag[0] ^= 1
	conn.EnqueueReply(t, encodeRoutableMessage(t, reply))

	// Verify that the response contains session info (this is a sanity check
	// on the test dummyConnector, not the code in dispather.go)
	select {
	case message := <-rsp.Recv():
		if message.GetSessionInfo() == nil {
			t.Errorf("Expected session info response but got %+v", message)
		}
	case <-ctx.Done():
		t.Errorf("Expected session info response but got %s", ctx.Err())
	}

	// Verify that we can the dispatcher didn't use the unauthenticated session
	// info to construct a session.
	if _, err := dispatcher.Send(ctx, req, connector.AuthMethodHMAC); err != protocol.ErrNoSession {
		t.Errorf("Unexpected error: %s", err)
	}
}

func TestCorruptedSessionInfo(t *testing.T) {
	dispatcher, conn := getTestSetup(t)
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), quiescentDelay)
	defer cancel()

	req := testCommand()
	req.ToDestination = &universal.Destination{
		SubDestination: &universal.Destination_Domain{Domain: testDomain + 1},
	}

	// Verify that we can send an unauthorized command to this domain
	rsp, err := dispatcher.Send(ctx, req, connector.AuthMethodNone)
	if err != nil {
		t.Fatalf("Error sending command: %s", err)
	}
	defer rsp.Close()

	reply := conn.SessionInfoReply(rsp, dispatcher.privateKey.PublicBytes())
	// Corrupt session info
	tag := reply.GetSignatureData().GetSessionInfoTag().GetTag()
	if len(tag) == 0 {
		t.Fatal("Reply didn't include a session info HMAC")
	}
	tag[0] ^= 1
	conn.EnqueueReply(t, encodeRoutableMessage(t, reply))

	// Verify that the response contains session info (this is a sanity check
	// on the test dummyConnector, not the code in dispather.go)
	select {
	case message := <-rsp.Recv():
		if message.GetSessionInfo() == nil {
			t.Errorf("Expected session info response but got %+v", message)
		}
	case <-ctx.Done():
		t.Errorf("Expected session info response but got %s", ctx.Err())
	}

	// Verify that we can the dispatcher didn't use the unauthenticated session
	// info to construct a session.
	if _, err := dispatcher.Send(ctx, req, connector.AuthMethodHMAC); err != protocol.ErrNoSession {
		t.Errorf("Unexpected error: %s", err)
	}

}

func TestDiscardUnauthenticatedSessionInfo(t *testing.T) {
	dispatcher, conn := getTestSetup(t)
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), quiescentDelay)
	defer cancel()

	req := testCommand()
	req.ToDestination = &universal.Destination{
		SubDestination: &universal.Destination_Domain{Domain: testDomain + 1},
	}

	// Verify that we can send an unauthorized command to this domain
	rsp, err := dispatcher.Send(ctx, req, connector.AuthMethodNone)
	if err != nil {
		t.Fatalf("Error sending command: %s", err)
	}
	defer rsp.Close()

	reply := conn.SessionInfoReply(rsp, dispatcher.privateKey.PublicBytes())
	// Remove authentication from session info
	reply.SubSigData = nil
	conn.EnqueueReply(t, encodeRoutableMessage(t, reply))

	// Verify that the response contains session info (this is a sanity check
	// on the test dummyConnector, not the code in dispather.go)
	select {
	case message := <-rsp.Recv():
		if message.GetSessionInfo() == nil {
			t.Errorf("Expected session info response but got %+v", message)
		}
	case <-ctx.Done():
		t.Errorf("Expected session info response but got %s", ctx.Err())
	}

	// Verify that we can the dispatcher didn't use the unauthenticated session
	// info to construct a session.
	if _, err := dispatcher.Send(ctx, req, connector.AuthMethodHMAC); err != protocol.ErrNoSession {
		t.Errorf("Unexpected error: %s", err)
	}
}

func TestVehicleUnreachable(t *testing.T) {
	dispatcher, conn := getTestSetup(t)
	defer conn.Close()

	conn.AckRequests = false

	ctx, cancel := context.WithTimeout(context.Background(), quiescentDelay)
	defer cancel()

	if _, err := dispatcher.Send(ctx, testCommand(), connector.AuthMethodNone); err != errTimeout {
		t.Errorf("Unexpected error: %s", err)
	}
}

func TestConnect(t *testing.T) {
	conn := newDummyConnector(t)
	defer conn.Close()

	key, err := authentication.NewECDHPrivateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Couldn't create private key: %s", err)
	}
	conn.Sleep()

	dispatcher, err := New(conn, key)
	if err != nil {
		t.Fatalf("Couldn't initialize dispatcher: %s", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), quiescentDelay)
	defer cancel()

	if err := dispatcher.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer dispatcher.Stop()

	if err := dispatcher.StartSessions(ctx, nil); err != context.DeadlineExceeded {
		t.Fatalf("Unexpected error: %s", err)
	}

	ctx, cancel = context.WithTimeout(context.Background(), quiescentDelay)
	defer cancel()

	if _, err := dispatcher.Send(ctx, testCommand(), connector.AuthMethodHMAC); err != protocol.ErrNoSession {
		t.Fatalf("Unexpected error: %s", err)
	}

	ctx, cancel = context.WithTimeout(context.Background(), quiescentDelay)
	defer cancel()

	conn.Wake()
	if err := dispatcher.StartSessions(ctx, nil); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	if _, err := dispatcher.Send(ctx, testCommand(), connector.AuthMethodHMAC); err != nil {
		t.Errorf("Unexpected error: %s", err)
	}
}

func TestWaitForAllSessions(t *testing.T) {
	conn := newDummyConnector(t)
	defer conn.Close()

	// Configure the Connector to only respond to the first of two handshakes
	conn.EnqueueSendError(nil)
	conn.EnqueueSendError(errDropMessage)

	key, err := authentication.NewECDHPrivateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Couldn't create private key: %s", err)
	}

	dispatcher, err := New(conn, key)
	if err != nil {
		t.Fatalf("Couldn't initialize dispatcher: %s", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), quiescentDelay)
	defer cancel()

	if err := dispatcher.Start(ctx); err != nil {
		t.Fatal(err)
	}

	if err := dispatcher.StartSessions(ctx, nil); err != context.DeadlineExceeded {
		t.Fatalf("Unexpected error: %s", err)
	}
}

func TestRetrySend(t *testing.T) {
	dispatcher, conn := getTestSetup(t)
	defer conn.Close()

	const errCount = 3
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	for i := 0; i < errCount; i++ {
		conn.EnqueueSendError(&protocol.CommandError{Err: errTimeout, PossibleSuccess: false, PossibleTemporary: true})
	}
	errFoo := errors.New("not enough pylons")
	conn.EnqueueSendError(&protocol.CommandError{Err: errFoo, PossibleSuccess: true, PossibleTemporary: true})

	req := testCommand()
	rsp, err := dispatcher.Send(ctx, req, connector.AuthMethodNone)
	if err == nil {
		rsp.Close()
	}
	if !errors.Is(err, errFoo) {
		t.Errorf("Unexpected error: %s", err)
	}
}

func TestSendTimeout(t *testing.T) {
	dispatcher, conn := getTestSetup(t)
	defer conn.Close()

	const errCount = 50
	ctx, cancel := context.WithTimeout(context.Background(), dispatcher.RetryInterval()/2)
	defer cancel()

	for i := 0; i < errCount; i++ {
		conn.EnqueueSendError(&protocol.CommandError{Err: errTimeout, PossibleSuccess: false, PossibleTemporary: true})
	}

	req := testCommand()
	rsp, err := dispatcher.Send(ctx, req, connector.AuthMethodNone)
	if err == nil {
		rsp.Close()
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Unexpected error: %s", err)
	}
	if e, ok := err.(protocol.Error); ok {
		if e.MayHaveSucceeded() {
			t.Errorf("Expected MayHaveSucceeded() to be false")
		}
		if !e.Temporary() {
			t.Errorf("Expected Temporary() to be true")
		}
	} else {
		t.Errorf("Expected Send to return an Error")
	}
}

func TestStopDispatcher(t *testing.T) {
	conn := newDummyConnector(t)
	defer conn.Close()

	key, err := authentication.NewECDHPrivateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Couldn't create private key: %s", err)
	}
	dispatcher, err := New(conn, key)
	if err != nil {
		t.Fatalf("Couldn't initialize dispatcher: %s", err)
	}

	ready := make(chan error, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := testCommand()
	if _, err = dispatcher.Send(ctx, req, connector.AuthMethodHMAC); !errors.Is(err, protocol.ErrNotConnected) {
		t.Errorf("Expected ErrNotConnected but got %s", err)
	}

	go func() {
		ready <- dispatcher.Start(ctx)
	}()

	dispatcher.Stop()
	select {
	case err := <-ready:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Errorf("Dispatcher Listen() goroutine didn't exit")
	}
}

func TestDoNotBlockOnResponder(t *testing.T) {
	// Verifies that if a Responder's inbox is full, sending another message to
	// that Responder does not prevent other Responders from receiving
	// messages.
	dispatcher, conn := getTestSetup(t)
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*quiescentDelay)
	defer cancel()

	rsp1, err := dispatcher.Send(ctx, testCommand(), connector.AuthMethodNone)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	rsp2, err := dispatcher.Send(ctx, testCommand(), connector.AuthMethodNone)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	reply1 := encodeRoutableMessage(t, replyWithPayload(rsp1, []byte("mailbox stuffer")))
	for i := 0; i < 2*receiverBufferSize; i++ {
		conn.EnqueueReply(t, reply1)
	}
	conn.EnqueueReply(t, encodeRoutableMessage(t, replyWithPayload(rsp2, []byte("I shouldn't be blocked"))))
	time.Sleep(quiescentDelay)

	select {
	case <-rsp2.Recv():
	case <-ctx.Done():
		t.Fatalf("Didn't receive message for second command: %s", err)
	}

	// Check that responderBufferSize messages (and no more!) arrived at the rsp1.
	for i := 0; i < receiverBufferSize; i++ {
		select {
		case <-rsp1.Recv():
		case <-ctx.Done():
			t.Fatalf("Didn't receive message for second command: %s", err)
		}
	}

	select {
	case <-rsp1.Recv():
		t.Fatalf("Received more messages than expected")
	case <-ctx.Done():
	}
}

func TestRequestSessionWithoutKey(t *testing.T) {
	conn := newDummyConnector(t)
	dispatcher, err := New(conn, nil)
	if err != nil {
		t.Fatalf("Couldn't initialize dispatcher: %s", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), quiescentDelay)
	defer cancel()

	if _, err := dispatcher.RequestSessionInfo(ctx, testDomain); err != protocol.ErrRequiresKey {
		t.Errorf("Expected ErrRequiresKey but got %s", err)
	}
}

func TestHandshakeWithoutKey(t *testing.T) {
	conn := newDummyConnector(t)
	defer conn.Close()

	dispatcher, err := New(conn, nil)
	if err != nil {
		t.Fatalf("Couldn't initialize dispatcher: %s", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), quiescentDelay)
	defer cancel()

	if err := dispatcher.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer dispatcher.Stop()

	if err := dispatcher.StartSession(ctx, testDomain); !errors.Is(err, protocol.ErrRequiresKey) {
		t.Errorf("Expected no key error but got %s", err)
	}
}

func TestNoValidHandshakeResponse(t *testing.T) {
	conn := newDummyConnector(t)
	defer conn.Close()

	key, err := authentication.NewECDHPrivateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dispatcher, err := New(conn, key)
	if err != nil {
		t.Fatal(err)
	}
	if err := dispatcher.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer dispatcher.Stop()

	const maxCallbacks = 5
	callbackCount := 0

	conn.callback = func(d *dummyConnector, message *universal.RoutableMessage) ([]byte, bool) {
		callbackCount++ // caller holds d.lock
		reply := initReply(message)
		reply.Payload = &universal.RoutableMessage_SessionInfo{}
		reply.SubSigData = &universal.RoutableMessage_SignatureData{
			SignatureData: &signatures.SignatureData{
				SigType: &signatures.SignatureData_SessionInfoTag{
					SessionInfoTag: &signatures.HMAC_Signature_Data{
						Tag: []byte("swordfish"),
					},
				},
			},
		}
		if callbackCount == maxCallbacks {
			reply.SignedMessageStatus = &universal.MessageStatus{
				SignedMessageFault: universal.MessageFault_E_MESSAGEFAULT_ERROR_UNKNOWN_KEY_ID,
			}
		}
		encoded, err := proto.Marshal(reply)
		if err != nil {
			panic(err)
		}
		return encoded, true
	}

	if err := dispatcher.StartSession(ctx, testDomain); !errors.Is(err, protocol.ErrKeyNotPaired) {
		t.Errorf("Expected key not paired but got %s", err)
	}
}

func TestCache(t *testing.T) {
	conn := newDummyConnector(t)
	key, err := authentication.NewECDHPrivateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Couldn't create private key: %s", err)
	}
	dispatcher, err := New(conn, key)
	if err != nil {
		t.Fatalf("Couldn't initialize dispatcher: %s", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), quiescentDelay)
	defer cancel()

	if err := dispatcher.Start(ctx); err != nil {
		t.Fatal(err)
	}
	err = dispatcher.StartSession(ctx, testDomain)
	if err != nil {
		t.Fatalf("Couldn't start session: %s", err)
	}

	cache := dispatcher.Cache()
	dispatcher.Stop()
	conn.Close()

	conn = newDummyConnector(t)
	defer conn.Close()

	dispatcher, err = New(conn, key)
	if err != nil {
		t.Fatal(err)
	}

	if err := dispatcher.LoadCache(cache); err != nil {
		t.Fatal(err)
	}

	if err := dispatcher.Start(ctx); err != nil {
		t.Fatal(err)
	}

	rsp, err := dispatcher.Send(ctx, testCommand(), connector.AuthMethodHMAC)
	if err != nil {
		t.Fatalf("Error getting response: %s", err)
	}

	conn.EnqueueReply(t, encodeRoutableMessage(t, replyWithPayload(rsp, []byte("hello world"))))

	select {
	case message := <-rsp.Recv():
		checkFault(t, message, universal.MessageFault_E_MESSAGEFAULT_ERROR_NONE)
	case <-ctx.Done():
		t.Errorf("Timed out waiting for response")
	}
}
