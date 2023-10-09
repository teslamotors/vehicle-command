package protocol

import (
	"crypto/ecdh"
	"errors"
	"io/fs"
	"testing"
)

func TestLoadPublicKey(t *testing.T) {
	expected, err := ecdh.P256().NewPublicKey([]byte{
		0x04, 0x2a, 0x01, 0xe3, 0x08, 0x84, 0x64, 0xb5, 0xe9, 0xf7, 0x2d, 0x68,
		0x79, 0x52, 0x27, 0xb2, 0xe9, 0x6b, 0xdc, 0x05, 0xb4, 0x79, 0x6d, 0xd5,
		0xa2, 0xcf, 0xc8, 0x6d, 0xa4, 0xde, 0x23, 0x37, 0xb8, 0xb2, 0xaf, 0x69,
		0x65, 0xea, 0xc9, 0x2e, 0x64, 0xc0, 0xfc, 0xdb, 0x8c, 0x5a, 0x07, 0xb7,
		0x64, 0xce, 0x6a, 0x01, 0xf4, 0x91, 0xef, 0xc5, 0x50, 0x88, 0xb5, 0xe1,
		0x98, 0x5f, 0x30, 0x4e, 0x63,
	})
	if err != nil {
		t.Fatal(err)
	}
	type testData struct {
		filename string
		ok       bool
	}

	tests := []testData{
		{"test/encrypted-pkcs8-invalid.pem", false},
		{"test/encrypted-pkcs8.pem", false},
		{"test/not-on-curve.pem", false},
		{"test/p521.pem", false},
		{"test/pkcs8-invalid.pem", false},
		{"test/pkcs8.pem", true},
		{"test/private-invalid.pem", false},
		{"test/private.pem", true},
		{"test/public-bad-b64.pem", false},
		{"test/public-invalid.pem", false},
		{"test/public.bin", true},
		{"test/public.hex", true},
		{"test/public.pem", true},
		{"test/rsa-public.pem", false},
		{"test/rsa.pem", false},
	}
	for _, test := range tests {
		pkey, err := LoadPublicKey(test.filename)
		t.Logf("%s -> %s", test.filename, err)
		if errors.Is(err, fs.ErrNotExist) {
			t.Errorf("Test file %s does not exist", test.filename)
			continue
		}
		if err == nil && !test.ok {
			t.Errorf("Expected %s to fail to load", test.filename)
		} else if err != nil && test.ok {
			t.Errorf("Expected %s to load, but got error %s", test.filename, err)
		} else if test.ok {
			if !pkey.Equal(expected) {
				t.Errorf("File %s did not contain the expected public key", test.filename)
			}
		}
	}
}
