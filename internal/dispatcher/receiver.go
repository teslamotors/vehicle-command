package dispatcher

import (
	"fmt"
	"time"

	universal "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/universalmessage"
)

var receiverBufferSize = 10

const (
	uuidLength      = 16
	challengeLength = 16
	addressLength   = 16
)

type receiverKey struct {
	address [addressLength]byte
	uuid    [uuidLength]byte
	domain  universal.Domain
}

func (r *receiverKey) String() string {
	return fmt.Sprintf("<%02x-%02x: %s>", r.address, r.uuid, r.domain)
}

// receiver represents a vehicle's pending response to a command.
type receiver struct {
	key           *receiverKey
	ch            chan *universal.RoutableMessage
	dispatcher    *Dispatcher
	requestSentAt time.Time
	lastActive    time.Time
}

// Recv returns a channel that receives responses to the command that created the receiver.
func (r *receiver) Recv() <-chan *universal.RoutableMessage {
	return r.ch
}

// Close tells the dispatcher to stop listening for responses to this command, freeing the
// corresponding resources.
func (r *receiver) Close() {
	if r.dispatcher != nil {
		r.dispatcher.closeHandler(r)
	}
}

// expired returns true if the request was sent long enough ago that any included session info
// should be discarded as stale.
func (r *receiver) expired(lifetime time.Duration) bool {
	return time.Now().After(r.requestSentAt.Add(lifetime))
}
