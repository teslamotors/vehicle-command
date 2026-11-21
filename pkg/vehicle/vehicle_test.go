package vehicle

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/teslamotors/vehicle-command/internal/dispatcher"
	"github.com/teslamotors/vehicle-command/pkg/connector"
	"github.com/teslamotors/vehicle-command/pkg/protocol"

	universal "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/universalmessage"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/vcsec"
)

type testReceiever struct {
	parent *testSender
}

func (r *testReceiever) Close() {}

func (r *testReceiever) UnauthenticatedFault() universal.MessageFault_E {
	r.parent.lock.Lock()
	defer r.parent.lock.Unlock()
	return r.parent.unauthenticatedFault
}

func (r *testReceiever) Recv() <-chan *universal.RoutableMessage {
	r.parent.lock.Lock()
	defer r.parent.lock.Unlock()
	if r.parent.fixedResponse != nil {
		select {
		case r.parent.ch <- r.parent.fixedResponse:
		default:
		}
	}
	return r.parent.ch
}

type testSender struct {
	lock          sync.Mutex
	listening     bool
	fixedResponse *universal.RoutableMessage
	ch            chan *universal.RoutableMessage
	// unauthenticatedFault is what receivers report as the fault of a dropped response.
	unauthenticatedFault universal.MessageFault_E

	// If SendError is set, Send() returns SendError. Otherwise, Send() returns
	// the first queued error (or nil if no errors are queued).
	SendError error
	errQueue  []error

	ConnectionErrors []error
}

func (s *testSender) StartSessions(_ context.Context, _ []universal.Domain) error {
	if len(s.ConnectionErrors) > 0 {
		err := s.ConnectionErrors[0]
		s.ConnectionErrors = s.ConnectionErrors[1:]
		return err
	}
	return nil
}

func (s *testSender) Cache() []dispatcher.CacheEntry {
	return nil
}

func (s *testSender) LoadCache(_ []dispatcher.CacheEntry) error {
	return nil
}

func (s *testSender) RetryInterval() time.Duration {
	return time.Millisecond
}

func (s *testSender) EnqueueError(err error) {
	s.lock.Lock()
	s.errQueue = append(s.errQueue, err)
	s.lock.Unlock()
}

func (s *testSender) EnqueueResponse(t *testing.T, message *universal.RoutableMessage) {
	t.Helper()
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.listening {
		t.Logf("Queueing response %+v", message)
		select {
		case s.ch <- message:
		default:
			t.Fatalf("Failed to enqueue response")
		}
	} else {
		t.Logf("Dropping response %+v", message)
	}
}

func (s *testSender) SetMaxLatency(_ time.Duration) {}

func newTestVehicle() (*Vehicle, *testSender) {
	dispatch := newTestSender()
	return &Vehicle{dispatcher: dispatch}, dispatch
}

func newTestSender() *testSender {
	return &testSender{
		ch: make(chan *universal.RoutableMessage, 5),
	}
}

func (s *testSender) Start(ctx context.Context) error {
	ready := make(chan struct{})
	go s.Listen(ready)
	select {
	case <-ready:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *testSender) Listen(ready chan<- struct{}) {
	s.lock.Lock()
	s.listening = true
	if ready != nil {
		close(ready)
	}
	s.lock.Unlock()
}

func (s *testSender) Stop() {
	s.lock.Lock()
	s.listening = false
	s.lock.Unlock()
}

func (s *testSender) Send(_ context.Context, _ *universal.RoutableMessage, _ connector.AuthMethod) (protocol.Receiver, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.SendError != nil {
		return nil, s.SendError
	}
	if len(s.errQueue) > 0 {
		err := s.errQueue[0]
		s.errQueue = s.errQueue[1:]
		return nil, err
	}
	return &testReceiever{parent: s}, nil
}

func TestVehicleStartSessionFailed(t *testing.T) {
	vehicle, dispatch := newTestVehicle()
	errFatal := errors.New("test: mine more minerals")
	dispatch.ConnectionErrors = []error{
		&protocol.CommandError{Err: errFatal, PossibleSuccess: false, PossibleTemporary: false},
		nil,
		nil,
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := vehicle.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	defer vehicle.Disconnect()
	if err := vehicle.StartSession(ctx, nil); !errors.Is(err, errFatal) {
		t.Errorf("Unexpected error: %s", err)
	}
}

func TestVehicleConnectionRetry(t *testing.T) {
	vehicle, dispatch := newTestVehicle()
	errFatal := errors.New("test: mine more minerals")
	dispatch.ConnectionErrors = []error{
		&protocol.CommandError{Err: errFatal, PossibleSuccess: false, PossibleTemporary: true},
		nil,
		nil,
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := vehicle.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	defer vehicle.Disconnect()
	if err := vehicle.StartSession(ctx, nil); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
}

func TestVehicleConnectionTimeout(t *testing.T) {
	vehicle, dispatch := newTestVehicle()
	errTransient := &protocol.CommandError{Err: errors.New("test: mine more minerals"), PossibleSuccess: false, PossibleTemporary: true}
	dispatch.ConnectionErrors = []error{
		errTransient,
		errTransient,
		errTransient,
		errTransient,
		errTransient,
		errTransient,
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	if err := vehicle.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	defer vehicle.Disconnect()
	cancel()
	if err := vehicle.StartSession(ctx, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("Unexpected error: %s", err)
	}
}

func TestVehicleSendError(t *testing.T) {
	vehicle, dispatch := newTestVehicle()
	if err := vehicle.Connect(context.Background()); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	defer vehicle.Disconnect()
	errTransient := errors.New("test: failed to synergize core competencies")
	errFatal := errors.New("test: mine more minerals")
	dispatch.EnqueueError(&protocol.CommandError{Err: errTransient, PossibleSuccess: false, PossibleTemporary: true})
	dispatch.EnqueueError(&protocol.CommandError{Err: errFatal, PossibleSuccess: false, PossibleTemporary: false})

	if _, err := vehicle.Send(context.Background(), universal.Domain_DOMAIN_VEHICLE_SECURITY, nil, connector.AuthMethodNone); !errors.Is(err, errFatal) {
		t.Errorf("Unexpected error: %s", err)
	}
}

func TestVehicleSendTimeout(t *testing.T) {
	vehicle, dispatch := newTestVehicle()
	if err := vehicle.Connect(context.Background()); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	defer vehicle.Disconnect()

	dispatch.SendError = &protocol.CommandError{
		Err:               errors.New("test: failed to pour libations"),
		PossibleSuccess:   false,
		PossibleTemporary: true,
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	_, err := vehicle.Send(ctx, universal.Domain_DOMAIN_VEHICLE_SECURITY, nil, connector.AuthMethodNone)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Unexpected error: %s", err)
	}
}

func TestVehicleRetryTimeout(t *testing.T) {
	vehicle, dispatch := newTestVehicle()
	if err := vehicle.Connect(context.Background()); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	defer vehicle.Disconnect()

	var message universal.RoutableMessage
	message.SignedMessageStatus = &universal.MessageStatus{
		OperationStatus:    universal.OperationStatus_E_OPERATIONSTATUS_WAIT,
		SignedMessageFault: universal.MessageFault_E_MESSAGEFAULT_ERROR_BUSY,
	}
	dispatch.fixedResponse = &message

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	_, err := vehicle.Send(ctx, universal.Domain_DOMAIN_VEHICLE_SECURITY, nil, connector.AuthMethodNone)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Unexpected error: %s", err)
	}
}

func TestVehicleNoResponseTimeout(t *testing.T) {
	vehicle, _ := newTestVehicle()
	if err := vehicle.Connect(context.Background()); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	defer vehicle.Disconnect()

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	_, err := vehicle.Send(ctx, universal.Domain_DOMAIN_VEHICLE_SECURITY, nil, connector.AuthMethodNone)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Unexpected error: %s", err)
	}
}

// checkTimeoutError verifies that err is a timeout that names fault, or that it doesn't mention a
// failed authentication when fault is MESSAGEFAULT_ERROR_NONE.
func checkTimeoutError(t *testing.T, err error, fault universal.MessageFault_E) {
	t.Helper()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Expected a timeout but got %v", err)
	}
	if !protocol.MayHaveSucceeded(err) {
		t.Errorf("Expected a timeout to be reported as possibly successful: %s", err)
	}
	if fault == universal.MessageFault_E_MESSAGEFAULT_ERROR_NONE {
		if strings.Contains(err.Error(), "authentication") {
			t.Errorf("Plain timeout mentions authentication: %s", err)
		}
	} else if !strings.Contains(err.Error(), fault.String()) {
		t.Errorf("Expected the error to name %s but got %s", fault, err)
	}
}

func TestVehicleTimeoutNamesUnauthenticatedFault(t *testing.T) {
	for _, fault := range []universal.MessageFault_E{
		universal.MessageFault_E_MESSAGEFAULT_ERROR_NONE,
		universal.MessageFault_E_MESSAGEFAULT_ERROR_RESPONSE_MTU_EXCEEDED,
	} {
		t.Run(fault.String(), func(t *testing.T) {
			vehicle, dispatch := newTestVehicle()
			if err := vehicle.Connect(context.Background()); err != nil {
				t.Fatalf("Unexpected error: %s", err)
			}
			defer vehicle.Disconnect()
			dispatch.unauthenticatedFault = fault

			ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
			defer cancel()

			_, err := vehicle.Send(ctx, universal.Domain_DOMAIN_INFOTAINMENT, nil, connector.AuthMethodNone)
			checkTimeoutError(t, err, fault)
		})
	}
}

func TestVCSECTimeoutNamesUnauthenticatedFault(t *testing.T) {
	_, dispatch := newTestVehicle()
	fault := universal.MessageFault_E_MESSAGEFAULT_ERROR_RESPONSE_MTU_EXCEEDED
	dispatch.unauthenticatedFault = fault

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	done := func(_ *vcsec.FromVCSECMessage) (bool, error) { return true, nil }
	_, err := readUntil(ctx, &testReceiever{parent: dispatch}, done)
	checkTimeoutError(t, err, fault)
}

func TestVehicleRetryFail(t *testing.T) {
	vehicle, dispatch := newTestVehicle()
	if err := vehicle.Connect(context.Background()); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	defer vehicle.Disconnect()

	retriableErrors := []universal.MessageFault_E{
		universal.MessageFault_E_MESSAGEFAULT_ERROR_BUSY,
		universal.MessageFault_E_MESSAGEFAULT_ERROR_TIMEOUT,
		universal.MessageFault_E_MESSAGEFAULT_ERROR_INVALID_SIGNATURE,
		universal.MessageFault_E_MESSAGEFAULT_ERROR_INVALID_TOKEN_OR_COUNTER,
		universal.MessageFault_E_MESSAGEFAULT_ERROR_INTERNAL,
		universal.MessageFault_E_MESSAGEFAULT_ERROR_INCORRECT_EPOCH,
		universal.MessageFault_E_MESSAGEFAULT_ERROR_TIME_EXPIRED,
		universal.MessageFault_E_MESSAGEFAULT_ERROR_TIME_TO_LIVE_TOO_LONG,
	}

	for _, fault := range retriableErrors {
		dispatch.EnqueueError(&protocol.RoutableMessageError{Code: fault})
	}
	dispatch.EnqueueError(&protocol.RoutableMessageError{Code: universal.MessageFault_E_MESSAGEFAULT_ERROR_INSUFFICIENT_PRIVILEGES})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := vehicle.Send(ctx, universal.Domain_DOMAIN_VEHICLE_SECURITY, nil, connector.AuthMethodNone)
	if errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Unexpected error: %s", err)
	}
}
