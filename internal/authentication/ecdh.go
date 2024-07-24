package authentication

// Why not crypto/ecdh or github.com/aead/ecdh?
//
// crypto/ecdh is specifically not chosen because the vehicle needs to use static ECDH keys.
// The crypto/ecdh package and github.com/aead/ecdh.KeyExchange interface aren't safe to use
// with static keys if you want to implement with an HSM. The interface would divulge
// long-term secrets to a compromised host machine.

// SharedKeySizeBytes is the length of the cryptographic key shared by a Signer and a Verifier.
const SharedKeySizeBytes = 16

// ECDHPrivateKey represents a local private key.
type ECDHPrivateKey interface {
	Exchange(remotePublicBytes []byte) (Session, error)
	PublicBytes() []byte
	SchnorrSignature(message []byte) ([]byte, error)
}
