package protocol

import (
	universal "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/universalmessage"
)

// A Receiver provides a channel for receiving universal.RoutableMessages from a remote peer.
//
// Typically a Receiver is returned by a function that sends a request to a vehicle, and the channel
// only carries a single response to that request.
type Receiver interface {
	Recv() <-chan *universal.RoutableMessage
	Close()
}
