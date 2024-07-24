package authentication

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"hash"
	"io"
	"math/big"
	"os"

	"github.com/teslamotors/vehicle-command/internal/schnorr"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/signatures"
)

// NativeSession implements the Session interface using native Go.
type NativeSession struct {
	gcm         cipher.AEAD
	key         []byte
	localPublic []byte
}

func (b *NativeSession) LocalPublicBytes() []byte {
	buff := make([]byte, len(b.localPublic))
	copy(buff, b.localPublic)
	return buff
}

func (b *NativeSession) Encrypt(plaintext, associatedData []byte) (nonce, ciphertext, tag []byte, err error) {
	if b.gcm == nil {
		err = errors.New("GCM context not initialized")
		return
	}
	nonce = make([]byte, b.gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return
	}
	length := len(plaintext)
	ciphertext = b.gcm.Seal(nil, nonce, plaintext, associatedData)
	tag = ciphertext[length:]
	ciphertext = ciphertext[:length]
	return
}

func (b *NativeSession) Decrypt(nonce, ciphertext, associatedData, tag []byte) (plaintext []byte, err error) {
	if b.gcm == nil {
		err = errors.New("GCM context not initialized")
		return
	}
	ctAndTag := make([]byte, 0, len(ciphertext)+len(tag))
	ctAndTag = append(ctAndTag, ciphertext...)
	ctAndTag = append(ctAndTag, tag...)
	plaintext, err = b.gcm.Open(nil, nonce, ctAndTag, associatedData)
	return
}

func (n *NativeSession) subkey(label []byte) []byte {
	kdf := hmac.New(sha256.New, n.key)
	kdf.Write(label)
	return kdf.Sum(nil)
}

func (b *NativeSession) NewHMAC(label string) hash.Hash {
	return hmac.New(sha256.New, b.subkey([]byte(label)))
}

func (b *NativeSession) SessionInfoHMAC(id, challenge, encodedInfo []byte) ([]byte, error) {
	meta := newMetadataHash(b.NewHMAC(labelSessionInfo))
	if err := meta.Add(signatures.Tag_TAG_SIGNATURE_TYPE, []byte{byte(signatures.SignatureType_SIGNATURE_TYPE_HMAC)}); err != nil {
		return nil, err
	}
	if err := meta.Add(signatures.Tag_TAG_PERSONALIZATION, id); err != nil {
		return nil, err
	}
	if err := meta.Add(signatures.Tag_TAG_CHALLENGE, challenge); err != nil {
		return nil, err
	}
	return meta.Checksum(encodedInfo), nil
}

type NativeECDHKey struct {
	*ecdsa.PrivateKey
}

func (n *NativeECDHKey) sharedSecret(publicBytes []byte) ([]byte, error) {
	x, y := elliptic.Unmarshal(elliptic.P256(), publicBytes)
	if x == nil {
		return nil, ErrInvalidPublicKey
	}

	sharedX, sharedY := elliptic.P256().ScalarMult(x, y, n.D.Bytes())

	if sharedX.Sign() == 0 && sharedY.Sign() == 0 {
		return nil, ErrInvalidPrivateKey
	}

	// Hash the shared x-coordinate.
	sharedSecret := make([]byte, (elliptic.P256().Params().BitSize+7)/8)
	sharedX.FillBytes(sharedSecret)
	return sharedSecret, nil
}

func (n *NativeECDHKey) Exchange(publicBytes []byte) (Session, error) {
	var err error
	sharedSecret, err := n.sharedSecret(publicBytes)
	if err != nil {
		return nil, err
	}
	// SHA1 is used to maintain compatibility with existing vehicle code, and
	// is safe to use in this context since we're just mapping a pseudo-random
	// curve point into a pseudo-random bit string.  Collision resistance isn't
	// needed.
	digest := sha1.Sum(sharedSecret)
	var session NativeSession
	session.key = digest[:SharedKeySizeBytes]

	block, err := aes.NewCipher(session.key)
	if err != nil {
		return nil, err
	}

	if session.gcm, err = cipher.NewGCM(block); err != nil {
		return nil, err
	}
	session.localPublic = n.PublicBytes()
	return &session, nil
}

func NewECDHPrivateKey(rng io.Reader) (ECDHPrivateKey, error) {
	if ecdsaKey, err := ecdsa.GenerateKey(elliptic.P256(), rng); err == nil {
		native := &NativeECDHKey{ecdsaKey}
		return native, nil
	} else {
		return nil, err
	}
}

func LoadExternalECDHKey(filename string) (ECDHPrivateKey, error) {
	pemBlock, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode([]byte(pemBlock))
	if block == nil {
		return nil, fmt.Errorf("%w: expected PEM encoding", ErrInvalidPrivateKey)
	}

	var ecdsaPrivateKey *ecdsa.PrivateKey

	if block.Type == "EC PRIVATE KEY" {
		ecdsaPrivateKey, err = x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
	} else {
		privateKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		var ok bool
		if ecdsaPrivateKey, ok = privateKey.(*ecdsa.PrivateKey); !ok {
			return nil, fmt.Errorf("%w: only elliptic curve keys supported", ErrInvalidPrivateKey)
		}
	}

	if ecdsaPrivateKey.Curve != elliptic.P256() {
		return nil, fmt.Errorf("%w: only NIST-P256 keys supported", ErrInvalidPrivateKey)
	}
	return &NativeECDHKey{ecdsaPrivateKey}, nil
}

func UnmarshalECDHPrivateKey(privateScalar []byte) ECDHPrivateKey {
	if len(privateScalar) != 32 {
		return nil
	}
	sk := NativeECDHKey{&ecdsa.PrivateKey{PublicKey: ecdsa.PublicKey{Curve: elliptic.P256()}}}
	var d big.Int
	sk.D = d.SetBytes(privateScalar)
	if sk.D.Cmp(elliptic.P256().Params().N) >= 0 {
		return nil
	}
	x, y := sk.PublicKey.Curve.ScalarBaseMult(privateScalar)
	sk.PublicKey.X = x
	sk.PublicKey.Y = y
	return &sk
}

func (n *NativeECDHKey) Public() *ecdsa.PublicKey {
	return &n.PublicKey
}

func (n *NativeECDHKey) PublicBytes() []byte {
	publicKey := n.Public()
	return elliptic.Marshal(publicKey, publicKey.X, publicKey.Y)
}

func (n *NativeECDHKey) SchnorrSignature(message []byte) ([]byte, error) {
	skey, err := n.PrivateKey.ECDH()
	if err != nil {
		return nil, err
	}
	return schnorr.Sign(skey, message)
}
