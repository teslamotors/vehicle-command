package authentication

import (
	"bytes"
	"crypto/sha1"
	"errors"
	"testing"
)

func phonePublicTestKey(t *testing.T) []byte {
	return []byte{
		0x04, 0x07, 0xfb, 0x60, 0xb6, 0x5b, 0x94, 0xe0, 0xde, 0x4a, 0x95,
		0x4c, 0x53, 0xbe, 0x10, 0x00, 0x3d, 0x9e, 0x69, 0x91, 0x8d, 0xed,
		0xfd, 0xa5, 0xf4, 0xe9, 0xef, 0xb9, 0xeb, 0xd8, 0xc5, 0xbd, 0x67,
		0x2a, 0x53, 0x99, 0x1c, 0x40, 0x68, 0x86, 0x5d, 0x5f, 0xb4, 0x4f,
		0x97, 0xf6, 0xce, 0xcf, 0x83, 0x98, 0xf2, 0x61, 0xdd, 0x1d, 0x7b,
		0xc6, 0x9b, 0xe6, 0x76, 0xaf, 0xdc, 0x8f, 0xfa, 0xcb, 0xcc,
	}
}

func TestLoadExternalECCKey(t *testing.T) {
	type testCase struct {
		filename string
		ok       bool
	}
	tests := []testCase{
		{"test_data/invalid_rsa.pem", false},
		{"test_data/valid_private_key.pem", true},
		{"test_data/invalid_curve.pem", false},
		{"test_data/does_not_exist.pem", false},
		{"test_data/empty.pem", false},
		{"test_data/public_key.pem", false},
		{"test_data/not_pem.pem", false},
		{"test_data/not_pkcs8.pem", true},
	}
	for _, test := range tests {
		key, err := LoadExternalECDHKey(test.filename)
		if (err == nil) != (key != nil) {
			t.Errorf("Inconsistent return values when loading %s", test.filename)
		} else if (err == nil) != test.ok {
			t.Errorf("Unexpected result when loading key from %s: %v", test.filename, err)
		}
	}
}

func TestSharedKey(t *testing.T) {
	// Test values are chosen so that the shared secret has a leading 0x00 byte
	privateKeyBytes := []byte{
		0x52, 0x60, 0xf8, 0xd6, 0x11, 0x38, 0x75, 0xd8, 0x6f, 0x8e, 0xe8,
		0xfe, 0xa3, 0x40, 0xdf, 0x1f, 0xfb, 0x40, 0xc6, 0x58, 0xb5, 0x45,
		0x5e, 0x8c, 0x33, 0xd7, 0x97, 0xc5, 0x3a, 0x41, 0xaf, 0xd3,
	}

	correctSharedSecret := []byte{
		0x00, 0x72, 0xd5, 0xb8, 0x15, 0x20, 0x7a, 0x04, 0xf0, 0xc7, 0x95, 0xfb,
		0xa0, 0xba, 0x9e, 0x8a, 0xdd, 0x3f, 0x1f, 0x57, 0x14, 0x8c, 0x51, 0xff,
		0xac, 0xe2, 0x2c, 0xa1, 0x5e, 0x6f, 0xd8, 0x45,
	}

	phonePublicKey := phonePublicTestKey(t)
	vehicleKey := UnmarshalECDHPrivateKey(privateKeyBytes).(*NativeECDHKey)
	if vehicleKey == nil {
		t.Fatalf("Error parsing private key")
	}
	session, err := vehicleKey.Exchange(phonePublicKey)
	if err != nil {
		t.Fatalf("Error deriving shared key: %s", err)
	}
	t.Logf("Shared key: %02x", session.(*NativeSession).key)
	digest := sha1.Sum(correctSharedSecret)
	if !bytes.Equal(session.(*NativeSession).key, digest[:16]) {
		t.Errorf("Failed to derive correct shared key!")
	}

	phonePublicKey[1] ^= 1
	_, err = vehicleKey.Exchange(phonePublicKey)
	if err != ErrInvalidPublicKey {
		t.Errorf("Didn't reject invalid curve point: %s", err)
	}

	zero := make([]byte, 65)
	zero[0] = 0x04
	_, err = vehicleKey.Exchange(zero)
	if err != ErrInvalidPublicKey {
		t.Errorf("Failed to reject point at infinity: %s", err)
	}
}

func TestZero(t *testing.T) {
	privateKeyBytes := make([]byte, 32)
	phonePublicKey := phonePublicTestKey(t)
	vehicleKey := UnmarshalECDHPrivateKey(privateKeyBytes).(*NativeECDHKey)
	if vehicleKey == nil {
		t.Fatalf("Error parsing private key")
	}
	_, err := vehicleKey.Exchange(phonePublicKey)
	if !errors.Is(err, ErrInvalidPrivateKey) {
		t.Errorf("Unexpected error: %s", err)
	}
}
