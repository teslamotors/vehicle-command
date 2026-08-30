package schnorr

import (
	"crypto/ecdh"
	"crypto/hmac"
	"crypto/sha256"

	"github.com/cronokirby/saferith"
)

var subgroupOrder = saferith.ModulusFromBytes(p256.Params().N.Bytes())

// DeterministicNonce implements RFC 6979, but only for P256/SHA256.
func DeterministicNonce(scalar []byte, messageHash [sha256.Size]byte) []byte {
	var asInt saferith.Nat

	// Steps below refer to the steps in RFC 6979 Section 3:
	// https://datatracker.ietf.org/doc/html/rfc6979#section-3
	//
	// Note that since this implementation only supports P256/SHA256:
	// hlen = qlen = 256 bits

	// Step (a), performed by caller: Set messageHash = SHA256(message)

	k := make([]byte, sha256.Size)
	v := make([]byte, sha256.Size)
	for i := 0; i < len(k); i++ {
		// Step (b): Set V = 0x01 0x01 ... 0x01
		v[i] = 0x01
		// Step (c): Set K = 0x00 0x00 ... 0x00
		k[i] = 0x00
	}

	// Set h1 := bits2octets(messageHash) for use in subsequent steps.
	asInt.SetBytes(messageHash[:])
	asInt.Mod(&asInt, subgroupOrder)
	h1 := asInt.Bytes()

	// Step (d): Set K = HMAC_K(V || 0x00 || scalar || h1)
	// Note: the RFC uses "x" to denote the scalar. We've already applied the bits2octets function
	// to h1  above.
	h := hmac.New(sha256.New, k)
	h.Write(v)
	h.Write([]byte{0x00})
	h.Write(scalar[:])
	h.Write(h1)
	k = h.Sum(nil)

	// Step (e): Set V = HMAC_K(V)
	h = hmac.New(sha256.New, k)
	h.Write(v)
	v = h.Sum(nil)

	// Step (f): Set K = HMAC_K(V || 0x01 || scalar || h1)
	h = hmac.New(sha256.New, k)
	h.Write(v)
	h.Write([]byte{0x01})
	h.Write(scalar[:])
	h.Write(h1)
	k = h.Sum(nil)

	// Step (g): V = HMAC_K(V)
	h = hmac.New(sha256.New, k)
	h.Write(v)
	v = h.Sum(nil)

	// Step (h): Loop until a proper value is found for the nonce (referred to as T in the RFC).
	var nonce saferith.Nat
	for {
		// Since hlen = qlen, we do not need a loop for step (h1) and (h2). We can simply set
		// V = HMAC_K(V)
		// ...and use V as the nonce bytes directly. That is, the value for T defined in the RFC is
		// always equal to V.
		h.Reset()
		h.Write(v)
		v = h.Sum(nil)

		// Step (h3): Compute nonce = bits2Int(T) = bits2Int(V)
		nonce.SetBytes(v[:])
		// ... if nonce is in the range [1,q-1], return v as the nonce.
		if _, _, lt := nonce.CmpMod(subgroupOrder); lt == 1 && nonce.EqZero() == 0 {
			return v[:]
		}

		// ... otherwise, compute:
		// K = HMAC_K(V || 0x00)
		h.Reset()
		h.Write(v)
		h.Write([]byte{0x00})
		k = h.Sum(nil)

		// V = HMAC_K(V)
		h = hmac.New(sha256.New, k)
		h.Write(v)
		v = h.Sum(nil)
		// ...and loop until we find a good value for the nonce.
	}
}

func Sign(skey *ecdh.PrivateKey, message []byte) ([]byte, error) {
	digest := sha256.Sum256(message)

	// Choose a nonce from [1, q-1], where q = |G| is the subgroup order. RFC uses a random nonce;
	// we use a deterministic nonce to avoid relying on an RNG.
	nonce, err := ecdh.P256().NewPrivateKey(DeterministicNonce(skey.Bytes(), digest))
	if err != nil {
		// Should never happen. DeterministicNonce should only return values in the appropriate
		// range.
		panic(err)
	}
	publicNonce := nonce.PublicKey().Bytes()

	// Variables defined as in RFC 8235, Section 3.3: https://www.rfc-editor.org/rfc/rfc8235.html#page-8
	var v, c, a saferith.Nat
	v.SetBytes(nonce.Bytes())
	c.SetBytes(challenge(publicNonce, skey.PublicKey().Bytes(), message))
	a.SetBytes(skey.Bytes())

	var r saferith.Nat
	// r = v - ac (mod q), where:
	//   - v is the (private) nonce
	//   - a is Alice's private key
	//   - c is the challenge value derived from Alice's public key, the public nonce V = vG, and
	//     the message being signed.
	r.ModSub(&v, r.ModMul(&a, &c, subgroupOrder), subgroupOrder)

	// The signature will be (V, r). V is an uncompressed curve point (2*ScalarLength) and r is a
	// ScalarLength.
	signature := make([]byte, 3*ScalarLength)

	// The first byte of the encoded public key is always 0x04. This indicates an uncompressed curve
	// point. It's redundant for our purposes, since this implementation doesn't support point
	// compression.
	copy(signature, publicNonce[1:])

	r.FillBytes(signature[2*ScalarLength : 3*ScalarLength])

	// Alice sends Bob (V, r) = (vG, r) = (vG, v - ac).
	//
	// Given a message, (V, r), and Alice's public key A = aG, Bob will compute c and confirm that
	// V = Ac + rG.
	//
	// If the signature is valid, then Ac + rG = (aG)c + rG = (ac + r)G = (ac + v - ac)G = vG = V.
	return signature[:], nil
}
