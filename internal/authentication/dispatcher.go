package authentication

import (
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/signatures"
)

// Dispatcher facilitates creating connections to multiple vehicles using the same ECDHPrivateKey.
type Dispatcher struct {
	ECDHPrivateKey
}

func (d *Dispatcher) Connect(verifierId []byte, sessionInfo *signatures.SessionInfo) (*Signer, error) {
	return NewSigner(d, verifierId, sessionInfo)
}

func (d *Dispatcher) ConnectAuthenticated(verifierId, challenge, encodedSessionInfo, tag []byte) (*Signer, error) {
	return NewAuthenticatedSigner(d, verifierId, challenge, encodedSessionInfo, tag)
}
