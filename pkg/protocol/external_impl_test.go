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

func (e *externalKey) PublicBytes() []byte                             { return nil }
func (e *externalKey) SchnorrSignature(message []byte) ([]byte, error) { return nil, nil }
func (e *externalKey) Exchange(remotePublicBytes []byte) (protocol.Session, error) {
	return &externalSession{}, nil
}

type externalSession struct{}

func (e *externalSession) SessionInfoHMAC(id, challenge, encodedInfo []byte) ([]byte, error) {
	return nil, nil
}
func (e *externalSession) Encrypt(plaintext, associatedData []byte) ([]byte, []byte, []byte, error) {
	return nil, nil, nil, nil
}
func (e *externalSession) Decrypt(nonce, ciphertext, associatedData, tag []byte) ([]byte, error) {
	return nil, nil
}
func (e *externalSession) LocalPublicBytes() []byte       { return nil }
func (e *externalSession) NewHMAC(label string) hash.Hash { return nil }

func TestExternalECDHPrivateKeyImplementation(t *testing.T) {
	var _ protocol.ECDHPrivateKey = (*externalKey)(nil)
	var _ protocol.Session = (*externalSession)(nil)
}
