package authentication

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/signatures"
	universal "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/universalmessage"
)

func mustDecodeHex(t *testing.T, value string) []byte {
	t.Helper()
	data, err := hex.DecodeString(value)
	if err != nil {
		t.Fatalf("invalid hex %q: %s", value, err)
	}
	return data
}

// TestProtocolDocAESGCMExample pins the worked AES-GCM encryption example in
// pkg/protocol/protocol.md (under "Authorizing commands"). If this test fails,
// the implementation and the documentation have drifted apart, and the example
// values in protocol.md need to be regenerated.
func TestProtocolDocAESGCMExample(t *testing.T) {
	const (
		docVIN        = "5YJ30123456789ABC"
		docEpoch      = "4c463f9cc0d3d26906e982ed224adde6"
		docExpiresAt  = 2655
		docCounter    = 7
		docMetadata   = "000105010103021135594a333031323334353637383941424303104c463f9cc0d3d26906e982ed224adde6040400000a5f050400000007070400000002ff"
		docKey        = "1b2fce19967b79db696f909cff89ea9a"
		docNonce      = "dbf79447fa156674dae1caed"
		docPlaintext  = "120452020801"
		docCiphertext = "38038e8c0f2e"
		docTag        = "c228e0ff64991481db3a7bbc133696c5"
	)

	meta := newMetadata()
	if err := meta.Add(signatures.Tag_TAG_SIGNATURE_TYPE, []byte{byte(signatures.SignatureType_SIGNATURE_TYPE_AES_GCM_PERSONALIZED)}); err != nil {
		t.Fatal(err)
	}
	if err := meta.Add(signatures.Tag_TAG_DOMAIN, []byte{byte(universal.Domain_DOMAIN_INFOTAINMENT)}); err != nil {
		t.Fatal(err)
	}
	if err := meta.Add(signatures.Tag_TAG_PERSONALIZATION, []byte(docVIN)); err != nil {
		t.Fatal(err)
	}
	if err := meta.Add(signatures.Tag_TAG_EPOCH, mustDecodeHex(t, docEpoch)); err != nil {
		t.Fatal(err)
	}
	if err := meta.AddUint32(signatures.Tag_TAG_EXPIRES_AT, docExpiresAt); err != nil {
		t.Fatal(err)
	}
	if err := meta.AddUint32(signatures.Tag_TAG_COUNTER, docCounter); err != nil {
		t.Fatal(err)
	}
	if err := meta.AddUint32(signatures.Tag_TAG_FLAGS, 1<<universal.Flags_FLAG_ENCRYPT_RESPONSE); err != nil {
		t.Fatal(err)
	}

	aad := meta.Checksum(nil)
	expectedAAD := sha256.Sum256(mustDecodeHex(t, docMetadata))
	if !bytes.Equal(aad, expectedAAD[:]) {
		t.Error("metadata serialization diverges from the byte string in protocol.md")
	}

	block, err := aes.NewCipher(mustDecodeHex(t, docKey))
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	sealed := gcm.Seal(nil, mustDecodeHex(t, docNonce), mustDecodeHex(t, docPlaintext), aad)

	ciphertext := sealed[:len(sealed)-gcm.Overhead()]
	tag := sealed[len(sealed)-gcm.Overhead():]
	if got := hex.EncodeToString(ciphertext); got != docCiphertext {
		t.Errorf("ciphertext diverges from protocol.md: got %s, want %s", got, docCiphertext)
	}
	if got := hex.EncodeToString(tag); got != docTag {
		t.Errorf("tag diverges from protocol.md: got %s, want %s", got, docTag)
	}
}
