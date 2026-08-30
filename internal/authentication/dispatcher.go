package authentication

import (
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/signatures"
)

// Dispatcher facilitates creating connections to multiple vehicles using the same ECDHPrivateKey.
type Dispatcher struct {
	ECDHPrivateKey
}

func (d *Dispatcher) Connect(verifierID []byte, sessionInfo *signatures.SessionInfo) (*Signer, error) {
	return NewSigner(d, verifierID, sessionInfo)
}

func (d *Dispatcher) ConnectAuthenticated(verifierID, challenge, encodedSessionInfo, tag []byte) (*Signer, error) {
	return NewAuthenticatedSigner(d, verifierID, challenge, encodedSessionInfo, tag)
}
