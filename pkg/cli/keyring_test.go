package cli

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"math/big"
	"testing"

	"github.com/teslamotors/vehicle-command/internal/authentication"

	"github.com/99designs/keyring"
)

// A valid P-256 private scalar whose first byte is zero, from issue #520. About one key in 256 looks like this.
var leadingZeroScalar = []byte{
	0x00, 0xd5, 0x0a, 0xf9, 0xaa, 0x67, 0xd9, 0x77, 0xd0, 0x2d, 0x78, 0xcb, 0xad, 0x72, 0xb2, 0xe9,
	0x1d, 0x2f, 0x76, 0x44, 0x03, 0x84, 0xbb, 0x84, 0xdc, 0x34, 0x7b, 0x97, 0xb5, 0x9b, 0x59, 0xa5,
}

func fileKeyringConfig(t *testing.T) *Config {
	return &Config{
		KeyringKeyName: "leading0",
		Backend: keyring.Config{
			ServiceName:      keyringServiceName,
			AllowedBackends:  []keyring.BackendType{keyring.FileBackend},
			FileDir:          t.TempDir(),
			FilePasswordFunc: func(string) (string, error) { return "test", nil },
		},
	}
}

func TestSaveKeyToKeyringRejectsInvalidScalars(t *testing.T) {
	valid := authentication.UnmarshalECDHPrivateKey(leadingZeroScalar).(*authentication.NativeECDHKey)
	n := elliptic.P256().Params().N
	tests := map[string]*big.Int{
		"zero":     big.NewInt(0),
		"negative": big.NewInt(-1),
		"N":        new(big.Int).Set(n),
		"33 bytes": new(big.Int).Lsh(big.NewInt(1), 256),
	}
	for name, d := range tests {
		t.Run(name, func(t *testing.T) {
			key := &authentication.NativeECDHKey{PrivateKey: &ecdsa.PrivateKey{PublicKey: valid.PublicKey, D: d}}
			if err := fileKeyringConfig(t).saveKeyToKeyring(key); err == nil {
				t.Fatalf("saveKeyToKeyring accepted the scalar %s", d)
			}
		})
	}
}

func TestKeyringRoundTripWithLeadingZeroScalar(t *testing.T) {
	key := authentication.UnmarshalECDHPrivateKey(leadingZeroScalar)
	if key == nil {
		t.Fatal("the test scalar should be a valid private key")
	}
	config := fileKeyringConfig(t)
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
