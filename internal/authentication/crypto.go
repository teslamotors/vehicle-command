package authentication

import (
	"errors"
	"fmt"
	"hash"
	"time"

	universal "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/universalmessage"
)

const (
	labelSessionInfo = "session info"
	labelMessageAuth = "authenticated command"
)

const (
	counterMax               = 0xFFFFFFFF
	epochIdLength            = 16
	maxSecondsWithoutCounter = 30
	windowSize               = 32 // Verifier.windowSize is uint64, so must be ≤ 64.
)

var (
	// ErrInvalidPublicKey is an Error raised when a remote peer provides an invalid public key.
	ErrInvalidPublicKey = newError(errCodeBadParameter, "invalid public key")
	// ErrInvalidPrivateKey indicates the local peer tried to load an unsupported or malformed
	// private key.
	ErrInvalidPrivateKey = errors.New("invalid private key")
)

// A Session allows encrypting/decrypting/authenticating data using a shared ECDH secret.
type Session interface {
	// Returns the session info HMAC tag for encodedInfo. The challenge is a Signer-provided
	// anti-replay value.
	SessionInfoHMAC(id, challenge, encodedInfo []byte) ([]byte, error)
	// Encrypt plaintext and generate a tag that can be used to authenticate
	// the ciphertext and associated data. The tag and ciphertext are part of
	// the same slice, but returned separately for convenience.
	Encrypt(plaintext, associatedData []byte) (nonce, ciphertext, tag []byte, err error)
	// Authenticate a ciphertext and its associated data using the tag, then
	// decrypt it and return the plaintext.
	Decrypt(nonce, ciphertext, associatedData, tag []byte) (plaintext []byte, err error)
	// Return the encoded local public key.
	LocalPublicBytes() []byte
	// Returns a hash.Hash context that can be used as a KDF rooted in the shared secret.
	NewHMAC(label string) hash.Hash
}

var epochLength = (1 << 30) * time.Second // var instead of const to facilitate testing

type InvalidSignatureError struct {
	Code        universal.MessageFault_E
	EncodedInfo []byte
	Tag         []byte
}

func (e *InvalidSignatureError) Error() string {
	return fmt.Sprintf("Invalid signature: %s", e.Code)
}

// Given the current time of some epoch, return the local time at which that
// epoch started.
func epochStartTime(epochTime uint32) time.Time {
	return time.Now().Add(-time.Second * time.Duration(epochTime))
}
