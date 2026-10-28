package vehicle

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/teslamotors/vehicle-command/pkg/connector"
	"github.com/teslamotors/vehicle-command/pkg/protocol"
	universal "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/universalmessage"
)

type cancellationReceiver struct {
	replies chan *universal.RoutableMessage
	closes  int
}

func (r *cancellationReceiver) Recv() <-chan *universal.RoutableMessage { return r.replies }
func (r *cancellationReceiver) Close()                                  { r.closes++ }

type cancellationSender struct {
	*testSender
	sent     chan struct{}
	receiver *cancellationReceiver
	sends    int
}

func (s *cancellationSender) Send(context.Context, *universal.RoutableMessage, connector.AuthMethod) (protocol.Receiver, error) {
	s.sends++
	if s.sends == 1 {
		close(s.sent)
	}
	return s.receiver, nil
}

func TestVehicleCancellationAfterSendPreservesPossibleSuccess(t *testing.T) {
	receiver := &cancellationReceiver{replies: make(chan *universal.RoutableMessage)}
	sender := &cancellationSender{testSender: newTestSender(), sent: make(chan struct{}), receiver: receiver}
	car := &Vehicle{dispatcher: sender}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, err := car.Send(ctx, universal.Domain_DOMAIN_VEHICLE_SECURITY, nil, connector.AuthMethodNone)
		result <- err
	}()
	select {
	case <-sender.sent:
	case <-time.After(time.Second):
		t.Fatal("command was not sent")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) || !protocol.MayHaveSucceeded(err) || protocol.ShouldRetry(err) {
			t.Fatalf("sent command cancellation = %v; want canceled, possible success, and no retry", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Send did not return after cancellation")
	}
	if sender.sends != 1 || receiver.closes != 1 {
		t.Fatalf("sends=%d receiver closes=%d, want one each", sender.sends, receiver.closes)
	}
}
