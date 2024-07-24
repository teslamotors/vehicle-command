package schnorr

import (
	"crypto/ecdh"
	"testing"
)

func testKey() *ecdh.PrivateKey {
	scalar := make([]byte, ScalarLength)
	scalar[0] = 3
	skey, _ := ecdh.P256().NewPrivateKey(scalar)
	return skey
}

func goodSig() ([]byte, []byte, []byte) {
	public := testKey().PublicKey().Bytes()
	message := []byte("hello world")
	signature := []byte{
		0x7c, 0xfd, 0xbe, 0xb5, 0xba, 0xa7, 0x30, 0x54, 0x04, 0x01, 0x55, 0x0b,
		0xde, 0xfa, 0x20, 0x97, 0x64, 0x53, 0xe8, 0x53, 0x9a, 0xe4, 0xb2, 0xf2,
		0x6c, 0xe3, 0x31, 0x25, 0x80, 0x1a, 0x08, 0xf9, 0x0e, 0xd2, 0x0c, 0x3d,
		0x84, 0x64, 0x97, 0xff, 0x82, 0xcc, 0x97, 0x72, 0xe3, 0xdb, 0x47, 0x03,
		0x98, 0x2f, 0x47, 0xbd, 0x0b, 0x0b, 0x89, 0xdf, 0xb9, 0xa4, 0x9c, 0xd2,
		0xe5, 0x24, 0x05, 0x46, 0x02, 0xb1, 0xe0, 0x5f, 0xbf, 0x95, 0xf5, 0x68,
		0x6f, 0xae, 0xa7, 0xa5, 0x80, 0x9e, 0xb9, 0x2f, 0x5e, 0xcc, 0x22, 0xea,
		0xe7, 0x4c, 0xec, 0xcc, 0x5e, 0x2a, 0x65, 0xdd, 0x67, 0xff, 0x20, 0xfc,
	}
	return public, message, signature
}

func TestVerify(t *testing.T) {
	pkey, message, signature := goodSig()
	if err := Verify(pkey, message, signature); err != nil {
		t.Error(err)
	}
}

func TestVerifyWrongMessage(t *testing.T) {
	pkey, message, signature := goodSig()
	message[0] ^= 1
	if err := Verify(pkey, message, signature); err != ErrInvalidSignature {
		t.Errorf("Expected ErrInvalidSignature but got %s", err)
	}
}

func TestVerifyWrongPublicKey(t *testing.T) {
	_, message, signature := goodSig()
	scalar := make([]byte, 32)
	scalar[0] = 4
	skey, err := ecdh.P256().NewPrivateKey(scalar)
	if err != nil {
		t.Fatal(err)
	}
	pkey := skey.PublicKey().Bytes()
	if err := Verify(pkey, message, signature); err != ErrInvalidSignature {
		t.Errorf("Expected ErrInvalidSignature but got %s", err)
	}
}

func TestNonceNotOnCurve(t *testing.T) {
	pkey, message, signature := goodSig()
	if err := Verify(pkey, message, signature); err != nil {
		t.Fatal(err)
	}
	signature[0] ^= 1
	if err := Verify(pkey, message, signature); err != ErrInvalidSignature {
		t.Errorf("Expected ErrInvalidSignature but got %s", err)
	}
	signature[0] ^= 1
	signature[ScalarLength] ^= 1
	if err := Verify(pkey, message, signature); err != ErrInvalidSignature {
		t.Errorf("Expected ErrInvalidSignature but got %s", err)
	}
}

func TestInvalidSignature(t *testing.T) {
	pkey, message, signature := goodSig()
	signature[len(signature)-1] ^= 1
	if err := Verify(pkey, message, signature); err != ErrInvalidSignature {
		t.Errorf("Expected ErrInvalidSignature but got %s", err)
	}
}

func TestPublicKeyNotOnCurve(t *testing.T) {
	pkey, message, signature := goodSig()
	pkey[0] ^= 1
	if err := Verify(pkey, message, signature); err != ErrInvalidPublicKey {
		t.Errorf("Expected ErrInvalidPublicKey but got %s", err)
	}
}

func TestZeroPublicKey(t *testing.T) {
	_, message, signature := goodSig()
	pkey := make([]byte, 65)
	pkey[0] = 0x04
	if err := Verify(pkey, message, signature); err != ErrInvalidPublicKey {
		t.Errorf("Expected ErrInvalidPublicKey but got %s", err)
	}
}

func TestSignatureTooShort(t *testing.T) {
	pkey, message, signature := goodSig()
	signature = signature[:len(signature)-1]
	if err := Verify(pkey, message, signature); err != ErrInvalidSignature {
		t.Errorf("Expected ErrInvalidSignature but got %s", err)
	}
}
