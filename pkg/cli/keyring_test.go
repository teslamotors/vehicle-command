package cli

import (
	"bytes"
	"crypto/elliptic"
	"math/big"
	"testing"

	"github.com/teslamotors/vehicle-command/internal/authentication"

	"github.com/99designs/keyring"
)

// A valid P-256 private scalar whose first byte is zero, from issue #520. About one key in 256 looks
// like this, and saveKeyToKeyring used to reject every one of them as "invalid private key".
var leadingZeroScalar = []byte{
	0x00, 0xd5, 0x0a, 0xf9, 0xaa, 0x67, 0xd9, 0x77, 0xd0, 0x2d, 0x78, 0xcb, 0xad, 0x72, 0xb2, 0xe9,
	0x1d, 0x2f, 0x76, 0x44, 0x03, 0x84, 0xbb, 0x84, 0xdc, 0x34, 0x7b, 0x97, 0xb5, 0x9b, 0x59, 0xa5,
}

func TestValidPrivateScalar(t *testing.T) {
	n := elliptic.P256().Params().N
	tests := []struct {
		name  string
		d     *big.Int
		valid bool
	}{
		{"zero", big.NewInt(0), false},
		{"negative", big.NewInt(-1), false},
		{"one", big.NewInt(1), true},
		{"leading zero byte", new(big.Int).SetBytes(leadingZeroScalar), true},
		{"N-1", new(big.Int).Sub(n, big.NewInt(1)), true},
		{"N", new(big.Int).Set(n), false},
		{"33 bytes", new(big.Int).Lsh(big.NewInt(1), 256), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validPrivateScalar(tt.d); got != tt.valid {
				t.Errorf("validPrivateScalar(%s) = %v, want %v", tt.d, got, tt.valid)
			}
		})
	}
}

func TestKeyringRoundTripWithLeadingZeroScalar(t *testing.T) {
	key := authentication.UnmarshalECDHPrivateKey(leadingZeroScalar)
	if key == nil {
		t.Fatal("the test scalar should be a valid private key")
	}
	config := &Config{
		KeyringKeyName: "leading0",
		Backend: keyring.Config{
			ServiceName:      keyringServiceName,
			AllowedBackends:  []keyring.BackendType{keyring.FileBackend},
			FileDir:          t.TempDir(),
			FilePasswordFunc: func(string) (string, error) { return "test", nil },
		},
	}
	if err := config.saveKeyToKeyring(key); err != nil {
		t.Fatalf("saving a key with a leading zero byte: %v", err)
	}
	loaded, err := config.LoadKeyFromKeyring()
	if err != nil {
		t.Fatalf("loading the key back: %v", err)
	}
	if !bytes.Equal(loaded.PublicBytes(), key.PublicBytes()) {
		t.Fatal("the key loaded from the keyring is not the key that was saved")
	}
}
