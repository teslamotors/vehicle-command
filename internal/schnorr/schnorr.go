package schnorr

// Implements Schnorr signatures using NIST P-256 and SHA-256.
//
// Some applications require sending message to offline vehicles. We want a system that
// accomplishes this and has the following properties:
//
//  1. Messages only need to be signed once for an entire fleet, instead of once per vehicle.
//  2. Fleet managers do not need to pair an additional key with each vehicle.
//  3. The scheme used to authenticate messages is safe to use alongside the existing ECDH/P256
//     protocol.
//
// The first requirement precludes using ECDH to derive a MAC key. The second requirement precludes
// bootstrapping the ECDH key to enroll a separate signature-verification key. The final requirement
// makes using the existing Fleet Manager ECDH/P256 key as an ECDSA/P256 key inadvisable, as
// interactions between the two schemes are difficult to analyze.
//
// Therefore we use Schnorr/P256 [1] with the existing ECDH/P256 key pair. The two should be free of
// interactions as long as we ensure domain separation in our hash function inputs [2], albeit in
// the random oracle model. In our case, the ECDH KDF uses SHA-1, while the Schnorr scheme uses
// SHA-256.
//
// Nonce generation is made deterministic using the RFC 6979 [3].
//
// [1] RFC 8235 https://www.rfc-editor.org/rfc/rfc8235.html
// [2] "On the Joint Security of Encryption and Signatures in EMV"
//     https://eprint.iacr.org/2011/615.pdf
// [3] RFC 6979 https://datatracker.ietf.org/doc/html/rfc6979#section-3

import (
	"crypto/elliptic"
	"crypto/sha256"
	"errors"
	"io"
	"math/big"
)

const ScalarLength = 32

var (
	p256 = elliptic.P256()
)

var (
	ErrInvalidSignature = errors.New("invalid signature")
	ErrInvalidPublicKey = errors.New("invalid public key")
)

func writeLengthValue(w io.Writer, buf []byte) {
	v := uint32(len(buf))
	w.Write([]byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)})
	w.Write(buf)
}

func challenge(publicNonce, senderPublicBytes, message []byte) []byte {
	h := sha256.New()
	writeLengthValue(h, elliptic.Marshal(p256, p256.Params().Gx, p256.Params().Gy))
	writeLengthValue(h, publicNonce)
	writeLengthValue(h, senderPublicBytes)
	writeLengthValue(h, message)
	return h.Sum(nil)
}

func Verify(publicKeyBytes, message, signature []byte) error {
	pX, pY := elliptic.Unmarshal(p256, publicKeyBytes)
	if pX == nil {
		return ErrInvalidPublicKey
	}
	if len(signature) != 3*ScalarLength {
		return ErrInvalidSignature
	}
	var vX, vY big.Int
	vX.SetBytes(signature[0:ScalarLength])
	vY.SetBytes(signature[ScalarLength : 2*ScalarLength])
	r := signature[2*ScalarLength:]
	c := challenge(append([]byte{0x04}, signature[:2*ScalarLength]...), publicKeyBytes, message)
	pX, pY = p256.ScalarMult(pX, pY, c)
	tempX, tempY := p256.ScalarBaseMult(r)
	pX, pY = p256.Add(tempX, tempY, pX, pY)
	if pX.Cmp(&vX) == 0 && pY.Cmp(&vY) == 0 {
		return nil
	} else {
		return ErrInvalidSignature
	}
}
