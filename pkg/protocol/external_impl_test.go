package protocol_test

import (
	"hash"
	"testing"

	"github.com/teslamotors/vehicle-command/pkg/protocol"
)

// externalKey is a no-op stub that satisfies protocol.ECDHPrivateKey from a
// package outside the vehicle-command module. The point of this test is purely
// structural: it would not compile prior to the Session alias being added,
// because the Exchange return type (authentication.Session) was not nameable
// from external code.
type externalKey struct{}

func (e *externalKey) PublicBytes() []byte                       { return nil }
func (e *externalKey) SchnorrSignature(_ []byte) ([]byte, error) { return nil, nil }
func (e *externalKey) Exchange(_ []byte) (protocol.Session, error) {
	return &externalSession{}, nil
}

type externalSession struct{}

func (e *externalSession) SessionInfoHMAC(_, _, _ []byte) ([]byte, error) {
	return nil, nil
}
func (e *externalSession) Encrypt(_, _ []byte) ([]byte, []byte, []byte, error) {
	return nil, nil, nil, nil
}
func (e *externalSession) Decrypt(_, _, _, _ []byte) ([]byte, error) {
	return nil, nil
}
func (e *externalSession) LocalPublicBytes() []byte   { return nil }
func (e *externalSession) NewHMAC(_ string) hash.Hash { return nil }

func TestExternalECDHPrivateKeyImplementation(_ *testing.T) {
	var _ protocol.ECDHPrivateKey = (*externalKey)(nil)
	var _ protocol.Session = (*externalSession)(nil)
}
