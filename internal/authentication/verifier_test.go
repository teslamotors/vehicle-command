package authentication

import (
	"bytes"
	"crypto/rand"
	"errors"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/carserver"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/signatures"
	universal "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/universalmessage"
)

func runVerifyTest(t *testing.T, verifier *Verifier, message *universal.RoutableMessage, expected universal.MessageFault_E, expectSessionInfo bool) {
	t.Helper()
	pt, err := verifier.Verify(message)
	gotSessionInfo := false
	if expected == errCodeOk {
		if err != nil {
			t.Errorf("Expected success but got %s", err)
		} else if !bytes.Equal(pt, testMessagePlaintext) {
			t.Errorf("Decrypted to %x instead of %s", pt, testMessagePlaintext)
		}
	} else {
		var observedErr universal.MessageFault_E
		if nativeErr, ok := err.(*Error); ok {
			observedErr = nativeErr.Code
		} else if sigErr, ok := err.(*InvalidSignatureError); ok {
			observedErr = sigErr.Code
			gotSessionInfo = true
		} else {
			t.Fatalf("Didn't get a valid error: %s", err)
		}

		if observedErr != expected {
			t.Errorf("Expected error code %d (%s) but got %s", expected, errCodeString(expected), observedErr)
		} else if pt != nil {
			t.Errorf("Invalid decryption didn't return nil plaintext")
		}
	}
	if expectSessionInfo != gotSessionInfo {
		t.Errorf("expectSessionInfo=%v, but gotSessionInfo=%v", expectSessionInfo, gotSessionInfo)
	}
}

func expectDetachedDecodingError(t *testing.T, prefix []byte, expected error) {
	t.Helper()
	verifier, signer := getGCMVerifierAndSigner(t)
	verifier.domain = universal.Domain_DOMAIN_INFOTAINMENT

	message := &universal.RoutableMessage{
		ToDestination: &universal.Destination{
			SubDestination: &universal.Destination_Domain{Domain: universal.Domain_DOMAIN_INFOTAINMENT},
		},
	}
	if err := signer.addDetachedAuthentication(message, prefix, []byte("secret"), time.Minute); err != nil {
		t.Fatalf("Error signing message: %s", err)
	}
	if _, err := verifier.Verify(message); !errors.Is(err, expected) {
		t.Fatalf("Expected detached decryption error but got %s", err)
	}
}

func TestInvalidDetachedEncryptionPrefixes(t *testing.T) {
	// "2: { 46: 5}"
	invalidWireType := []byte{0x12, 0x03, 0xf0, 0x02, 0x05}

	// 2: { 46: { 1: "secret" 2: "extra" }}
	multipleFields := []byte{0x12, 0x10, 0xf2, 0x02, 0x0d, 0x08, 0x73, 0x65, 0x63, 0x72, 0x65, 0x74,
		0x10, 0x65, 0x78, 0x74, 0x72, 0x61}

	// 2: { 46: { 1: "secret" }}
	extraBytes := []byte{0x12, 0x0a, 0xf2, 0x02, 0x07, 0x08, 0x73, 0x65, 0x63, 0x72, 0x65, 0x74}

	extraBytes = append(extraBytes, 0x00, 0x01, 0x02)
	tests := map[string][]byte{
		"empty":             {},
		"invalid protobuf":  {0xFF, 0xFF, 0xFF},
		"invalid wire type": invalidWireType,
		"multiple fields":   multipleFields,
		"extra bytes":       extraBytes,
	}
	for name, payload := range tests {
		t.Run(name, func(t *testing.T) {
			expectDetachedDecodingError(t, payload, ErrInvalidDetachedPrefix)
		})
	}
}

func TestDetachPrefixTooLong(t *testing.T) {
	// 2: { 46: { 1: "secret" }}
	validCommand := []byte{0x12, 0x0a, 0xf2, 0x02, 0x07, 0x08, 0x73, 0x65, 0x63, 0x72, 0x65, 0x74}
	verifier, signer := getGCMVerifierAndSigner(t)
	verifier.domain = universal.Domain_DOMAIN_INFOTAINMENT

	message := &universal.RoutableMessage{
		ToDestination: &universal.Destination{
			SubDestination: &universal.Destination_Domain{Domain: universal.Domain_DOMAIN_INFOTAINMENT},
		},
		Payload: &universal.RoutableMessage_ProtobufMessageAsBytes{
			ProtobufMessageAsBytes: validCommand,
		},
	}

	if err := signer.EncryptDetach(message, time.Minute); err != nil {
		t.Fatalf("Error signing message: %s", err)
	}

	message.GetSignatureData().GetAES_GCM_DetachedData().Prefix = bytes.Repeat([]byte{0xFF}, 300)

	if _, err := verifier.Verify(message); !errors.Is(err, ErrMetadataFieldTooLong) {
		t.Fatalf("Expected detached decryption error but got %s", err)
	}
}

func TestDetachedEncryption(t *testing.T) {
	verifier, signer := getGCMVerifierAndSigner(t)
	verifier.domain = universal.Domain_DOMAIN_INFOTAINMENT
	// 2: { 46: { 1: "secret" }}
	command := []byte{0x12, 0x0a, 0xf2, 0x02, 0x07, 0x08, 0x73, 0x65, 0x63, 0x72, 0x65, 0x74}
	// "2: { 46: {}}"
	plaintextPrefix := []byte{0x12, 0x03, 0xf2, 0x02, 0x00}

	message := &universal.RoutableMessage{
		ToDestination: &universal.Destination{
			SubDestination: &universal.Destination_Domain{Domain: universal.Domain_DOMAIN_INFOTAINMENT},
		},
		Payload: &universal.RoutableMessage_ProtobufMessageAsBytes{
			ProtobufMessageAsBytes: command,
		},
	}
	if err := signer.EncryptDetach(message, time.Second); err != nil {
		t.Fatalf("Error signing message: %s", err)
	}
	encoded, err := proto.Marshal(message)
	if err != nil {
		t.Fatalf("Error re-encoding message: %s", err)
	}
	if bytes.Contains(encoded, []byte("secret")) {
		t.Errorf("Message was not encrypted: %x", encoded)
	}
	if !bytes.Equal(message.GetSignatureData().GetAES_GCM_DetachedData().GetPrefix(), plaintextPrefix) {
		t.Errorf("Message prefix was modified: %x", message.GetSignatureData().GetAES_GCM_DetachedData())
	}
	plaintext, err := verifier.Verify(message)
	if err != nil {
		t.Fatalf("Error verifying message: %s", err)
	}
	if !bytes.Equal(plaintext, command) {
		t.Errorf("Didn't recover original plaintext: %x", plaintext)
	}

	message.GetSignatureData().GetAES_GCM_DetachedData().Counter = 1
	runVerifyTest(t, verifier, message, errCodeInvalidToken, true)

	message.GetSignatureData().GetAES_GCM_DetachedData().Counter = 10
	message.GetSignatureData().GetAES_GCM_DetachedData().Tag[0] ^= 1
	runVerifyTest(t, verifier, message, errCodeInvalidSignature, true)
}

func TestDetachedNonceLength(t *testing.T) {
	verifier, signer := getGCMVerifierAndSigner(t)
	verifier.domain = universal.Domain_DOMAIN_INFOTAINMENT
	message := &universal.RoutableMessage{
		ToDestination: &universal.Destination{
			SubDestination: &universal.Destination_Domain{Domain: universal.Domain_DOMAIN_INFOTAINMENT},
		},
		Payload: &universal.RoutableMessage_ProtobufMessageAsBytes{
			ProtobufMessageAsBytes: []byte{0x12, 0x0a, 0xf2, 0x02, 0x07, 0x08, 0x73, 0x65, 0x63, 0x72, 0x65, 0x74},
		},
	}
	if err := signer.EncryptDetach(message, time.Minute); err != nil {
		t.Fatalf("Error signing message: %s", err)
	}
	sigData := message.GetSignatureData().GetAES_GCM_DetachedData()
	for _, nonce := range [][]byte{append(sigData.Nonce, 3), {3}, nil} {
		sigData.Nonce = nonce
		runVerifyTest(t, verifier, message, universal.MessageFault_E_MESSAGEFAULT_ERROR_IV_INCORRECT_LENGTH, true)
	}
}

func TestEncryptDetachRejectsInvalidCommands(t *testing.T) {
	_, signer := getGCMVerifierAndSigner(t)
	tests := map[string]struct {
		domain  universal.Domain
		payload []byte
	}{
		"missing payload":    {universal.Domain_DOMAIN_INFOTAINMENT, nil},
		"unsupported domain": {universal.Domain_DOMAIN_BROADCAST, []byte{0x12, 0x00}},
		"varint command":     {universal.Domain_DOMAIN_VEHICLE_SECURITY, []byte{0x10, 0x01}},
		"too shallow":        {universal.Domain_DOMAIN_INFOTAINMENT, []byte{0x12, 0x00}},
		"trailing data":      {universal.Domain_DOMAIN_VEHICLE_SECURITY, []byte{0x0a, 0x00, 0x10, 0x01}},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			message := &universal.RoutableMessage{
				ToDestination: &universal.Destination{
					SubDestination: &universal.Destination_Domain{Domain: test.domain},
				},
			}
			if test.payload != nil {
				message.Payload = &universal.RoutableMessage_ProtobufMessageAsBytes{ProtobufMessageAsBytes: test.payload}
			}
			original := proto.Clone(message)
			if err := signer.EncryptDetach(message, time.Minute); err == nil {
				t.Fatal("Expected error")
			}
			if !proto.Equal(original, message) {
				t.Errorf("Message was modified on error: %v", message)
			}
		})
	}
}

func TestValidGCMEncryption(t *testing.T) {
	verifier, signer := getGCMVerifierAndSigner(t)
	message := getTestMessage()
	if err := signer.Encrypt(message, time.Minute); err != nil {
		t.Fatalf("Error signing message: %s", err)
	}
	runVerifyTest(t, verifier, message, errCodeOk, false)
	runVerifyTest(t, verifier, message, errCodeInvalidToken, true) // Replay attack
}

func TestGCMFlags(t *testing.T) {
	verifier, signer := getGCMVerifierAndSigner(t)
	message := getTestMessage()
	if err := signer.Encrypt(message, time.Minute); err != nil {
		t.Fatalf("Error signing message: %s", err)
	}
	message.Flags = 1
	runVerifyTest(t, verifier, message, errCodeInvalidSignature, true)
	message.Flags = 0
	runVerifyTest(t, verifier, message, errCodeOk, false)
}

func TestGCMBroadcast(t *testing.T) {
	verifier, signer := getGCMVerifierAndSigner(t)
	message := getTestMessage()
	if err := signer.Encrypt(message, time.Minute); err != nil {
		t.Fatalf("Error signing message: %s", err)
	}
	verifier.domain++
	runVerifyTest(t, verifier, message, errCodeInvalidDomain, false)
	verifier.domain = universal.Domain_DOMAIN_BROADCAST
	runVerifyTest(t, verifier, message, errCodeOk, false)
}

func TestGCMInvalidVerifierName(t *testing.T) {
	verifier, signer := getGCMVerifierAndSigner(t)
	signer.verifierName = make([]byte, 256)
	challenge := make([]byte, 5)
	info, tag, err := verifier.SignedSessionInfo(challenge)
	if err != nil {
		t.Errorf("Failed to generated signed session info")
	}
	err = signer.UpdateSignedSessionInfo(challenge, info, tag)
	if err != ErrMetadataFieldTooLong {
		t.Errorf("Expected to get field to long error but got %s", err)
	}
}

func TestInvalidVerifierInputs(t *testing.T) {
	verifier, signer := getGCMVerifierAndSigner(t)

	longName := make([]byte, 500)
	if _, _, err := verifier.SignedSessionInfo(longName); err != ErrMetadataFieldTooLong {
		t.Errorf("Failed to reject illegally long challenge")
	}

	sk, err := NewECDHPrivateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate ECDH key: %s", err)
	}
	if _, err := NewVerifier(sk, longName, universal.Domain_DOMAIN_VEHICLE_SECURITY, signer.session.LocalPublicBytes()); err != ErrMetadataFieldTooLong {
		t.Errorf("Failed to reject illegally long verifier ID")
	}
	if _, err := NewSigner(sk, longName, nil); err != ErrMetadataFieldTooLong {
		t.Errorf("Failed to reject illegally long verifier ID")
	}
}

func TestInvalidVerifierPublicKey(t *testing.T) {
	verifierKey, signerKey := getVerifierAndSignerKeys(t)
	verifier, err := NewVerifier(verifierKey, []byte("foo"), universal.Domain_DOMAIN_VEHICLE_SECURITY, signerKey.PublicBytes())
	if err != nil {
		t.Fatalf("Error creating verifier: %s", err)
	}
	info, err := verifier.SessionInfo()
	if err != nil {
		t.Fatalf("Error creating session info: %s", err)
	}
	if _, err = NewSigner(signerKey, []byte("foo"), info); err != nil {
		t.Fatalf("Error setting up Signer: %s", err)
	}
	info.PublicKey[1] ^= 1
	_, err = NewSigner(signerKey, []byte("foo"), info)
	checkError(t, err, errCodeBadParameter)
}

func TestInvalidSignerPublicKey(t *testing.T) {
	verifierKey, signerKey := getVerifierAndSignerKeys(t)
	signerPublicBytes := signerKey.PublicBytes()
	signerPublicBytes[1] ^= 0x01
	_, err := NewVerifier(verifierKey, []byte("foo"), universal.Domain_DOMAIN_VEHICLE_SECURITY, signerPublicBytes)
	checkError(t, err, errCodeBadParameter)
}

func TestGCMMissingDestination(t *testing.T) {
	verifier, signer := getGCMVerifierAndSigner(t)
	message := getTestMessage()
	if err := signer.Encrypt(message, time.Minute); err != nil {
		t.Fatalf("Error signing message: %s", err)
	}
	message.ToDestination = nil
	runVerifyTest(t, verifier, message, errCodeInvalidDomain, false)
}

func TestGCMImplicitEpoch(t *testing.T) {
	verifier, signer := getGCMVerifierAndSigner(t)
	message := getTestMessage()
	if err := signer.Encrypt(message, time.Minute); err != nil {
		t.Fatalf("Error signing message: %s", err)
	}
	message.GetSignatureData().GetAES_GCM_PersonalizedData().Epoch = nil
	runVerifyTest(t, verifier, message, errCodeOk, false)
}

func TestGCMDomainOutOfRange(t *testing.T) {
	verifier, signer := getGCMVerifierAndSigner(t)
	message := getTestMessage()
	if err := signer.Encrypt(message, time.Minute); err != nil {
		t.Fatalf("Error signing message: %s", err)
	}
	verifier.domain = universal.Domain_DOMAIN_BROADCAST
	message.GetToDestination().SubDestination = &universal.Destination_Domain{Domain: universal.Domain(256)}
	runVerifyTest(t, verifier, message, errCodeInvalidDomain, false)
}

func TestGCMUsesRoutableAddress(t *testing.T) {
	verifier, signer := getGCMVerifierAndSigner(t)
	message := getTestMessage()
	if err := signer.Encrypt(message, time.Minute); err != nil {
		t.Fatalf("Error signing message: %s", err)
	}
	message.GetToDestination().SubDestination = &universal.Destination_RoutingAddress{
		RoutingAddress: []byte{1, 2, 3},
	}
	runVerifyTest(t, verifier, message, errCodeInvalidDomain, false)
}

func decodeInfo(encodedInfo []byte) *signatures.SessionInfo {
	var info signatures.SessionInfo
	if err := proto.Unmarshal(encodedInfo, &info); err != nil {
		panic(err)
	}
	return &info
}

func TestGCMEpochRotation(t *testing.T) {
	verifier, signer := getGCMVerifierAndSigner(t)
	challenge := []byte{0, 1, 2, 3, 4, 5, 6, 7}
	oldEncodedInfo, _, err := verifier.SignedSessionInfo(challenge)
	if err != nil {
		t.Fatalf("Error getting session: %s", err)
	}
	signer.counter = 0xFFFFFFFE
	message := getTestMessage()
	if err := signer.Encrypt(message, time.Minute); err != nil {
		t.Fatalf("Error signing message: %s", err)
	}
	runVerifyTest(t, verifier, message, errCodeOk, false)
	newEncodedInfo, _, err := verifier.SignedSessionInfo(challenge)
	if err != nil {
		t.Fatalf("Error getting session: %s", err)
	}

	oldSession := decodeInfo(oldEncodedInfo)
	newSession := decodeInfo(newEncodedInfo)

	if bytes.Equal(oldSession.Epoch[:], newSession.Epoch[:]) {
		t.Errorf("Session didn't rotate after max counter value")
	}
}

func TestGCMOutOfOrderMessage(t *testing.T) {
	verifier, signer := getGCMVerifierAndSigner(t)
	message := getTestMessage()
	if err := signer.Encrypt(message, time.Minute); err != nil {
		t.Fatalf("Error signing message: %s", err)
	}
	message2 := getTestMessage()
	if err := signer.Encrypt(message2, time.Minute); err != nil {
		t.Fatalf("Error signing message: %s", err)
	}
	runVerifyTest(t, verifier, message2, errCodeOk, false)
	runVerifyTest(t, verifier, message, universal.MessageFault_E_MESSAGEFAULT_ERROR_TIME_TO_LIVE_TOO_LONG, true)
}

func TestEpochChange(t *testing.T) {
	id := []byte("mycar")
	challenge := []byte("challenge")
	verifierKey, signerKey := getVerifierAndSignerKeys(t)
	verifier, err := NewVerifier(verifierKey, id, universal.Domain_DOMAIN_VEHICLE_SECURITY, signerKey.PublicBytes())
	if err != nil {
		t.Fatalf("Failed to initialize verifier: %s", err)
	}

	info, tag, err := verifier.SignedSessionInfo(challenge)
	if err != nil {
		t.Fatalf("Couldn't get session info: %s", err)
	}

	signer, err := NewAuthenticatedSigner(signerKey, id, challenge, info, tag)
	if err != nil {
		t.Fatalf("Rejected valid session info: %s", err)
	}

	message := getTestMessage()
	if err := signer.Encrypt(message, time.Second); err != nil {
		t.Fatalf("Error signing message: %s", err)
	}
	runVerifyTest(t, verifier, message, errCodeOk, false)

	// Vehicle resets (but keeps same key)
	verifier, err = NewVerifier(verifierKey, id, universal.Domain_DOMAIN_VEHICLE_SECURITY, signerKey.PublicBytes())
	if err != nil {
		t.Fatalf("Failed to initialize verifier: %s", err)
	}

	// First command fails because signer is out of sync
	message = getTestMessage()
	if err := signer.Encrypt(message, time.Second); err != nil {
		t.Fatalf("Error signing message: %s", err)
	}
	runVerifyTest(t, verifier, message, errCodeInvalidEpoch, true)

	// Signer resyncs with Verifier
	challenge[0] ^= 1
	info, tag, err = verifier.SignedSessionInfo(challenge)
	if err != nil {
		t.Fatalf("Couldn't get session info: %s", err)
	}

	// Resync resolves errors
	if err := signer.UpdateSignedSessionInfo(challenge, info, tag); err != nil {
		t.Fatalf("Rejected valid session info: %s", err)
	}
	message = getTestMessage()
	if err := signer.Encrypt(message, time.Second); err != nil {
		t.Fatalf("Error signing message: %s", err)
	}
	runVerifyTest(t, verifier, message, errCodeOk, false)
}

func TestEpochCopied(t *testing.T) {
	verifier, signer := getGCMVerifierAndSigner(t)
	message := getTestMessage()
	if err := signer.Encrypt(message, epochLength); err != nil {
		t.Fatalf("Error signing message: %s", err)
	}
	signer.epoch[0] ^= 1
	runVerifyTest(t, verifier, message, errCodeOk, false)
}

func TestGCMCorruptedCiphertext(t *testing.T) {
	verifier, signer := getGCMVerifierAndSigner(t)
	message := getTestMessage()
	if err := signer.Encrypt(message, time.Minute); err != nil {
		t.Fatalf("Error signing message: %s", err)
	}
	msg := message.GetProtobufMessageAsBytes()
	msg[0] ^= 1
	runVerifyTest(t, verifier, message, errCodeInvalidSignature, true)
}

func TestGCMCorruptedMetadata(t *testing.T) {
	verifier, signer := getGCMVerifierAndSigner(t)
	message := getTestMessage()
	if err := signer.Encrypt(message, time.Minute); err != nil {
		t.Fatalf("Error signing message: %s", err)
	}
	message.GetSignatureData().GetAES_GCM_PersonalizedData().Counter++
	runVerifyTest(t, verifier, message, errCodeInvalidSignature, true)
}

func TestGCMExpired(t *testing.T) {
	verifier, signer := getGCMVerifierAndSigner(t)
	message := getTestMessage()
	if err := signer.Encrypt(message, time.Minute); err != nil {
		t.Fatalf("Error signing message: %s", err)
	}
	verifier.timeZero = verifier.timeZero.Add(-time.Hour)
	runVerifyTest(t, verifier, message, errCodeExpired, true)
}

func TestGCMInvalidEpoch(t *testing.T) {
	verifier, signer := getGCMVerifierAndSigner(t)
	message := getTestMessage()
	if err := signer.Encrypt(message, time.Minute); err != nil {
		t.Fatalf("Error signing message: %s", err)
	}
	verifier.epoch[0] ^= 1
	runVerifyTest(t, verifier, message, errCodeInvalidEpoch, true)
}

func TestGCMInvalidTime(t *testing.T) {
	verifier, signer := getGCMVerifierAndSigner(t)
	message := getTestMessage()
	epochLength += time.Hour
	if err := signer.Encrypt(message, epochLength-time.Minute); err != nil {
		t.Fatalf("Error signing message: %s", err)
	}
	epochLength -= time.Hour
	runVerifyTest(t, verifier, message, errCodeBadParameter, true)
}

func TestGCMKnown(t *testing.T) {
	encodedMessage := []byte{
		0x32, 0x02, 0x08, 0x02, 0x3a, 0x02, 0x08, 0x00, 0x52, 0x59, 0x29, 0x6d,
		0x58, 0x7d, 0x13, 0x0d, 0x34, 0xd7, 0x4d, 0x6d, 0x5c, 0x62, 0x8c, 0x73,
		0xc1, 0xf8, 0xef, 0x99, 0xcc, 0x4a, 0xe1, 0xc9, 0x5b, 0x97, 0x67, 0x14,
		0x74, 0x98, 0x0c, 0xcc, 0x79, 0x46, 0xa9, 0x0a, 0x2f, 0x79, 0x71, 0xa5,
		0xc8, 0x37, 0xb9, 0x6c, 0x9c, 0xc1, 0xe8, 0x07, 0x5d, 0x1a, 0xab, 0x92,
		0xbc, 0x85, 0x57, 0xb7, 0xfd, 0xf2, 0xdc, 0xf9, 0xdd, 0xbb, 0x90, 0xe2,
		0x36, 0x24, 0x6d, 0xb6, 0x99, 0x78, 0x8e, 0x58, 0x5e, 0x8b, 0x0e, 0xa8,
		0x47, 0x52, 0xe0, 0x09, 0x0c, 0xc8, 0x0c, 0x43, 0x84, 0xd2, 0x7c, 0xa6,
		0xfc, 0xdd, 0x21, 0x6a, 0x47, 0x0a, 0x06, 0x12, 0x04, 0xbb, 0x0c, 0xa3,
		0x71, 0x2a, 0x3d, 0x0a, 0x10, 0xea, 0xab, 0xe3, 0x01, 0xb4, 0xb4, 0xa1,
		0x24, 0x31, 0x18, 0xa4, 0x08, 0x25, 0x22, 0x01, 0x15, 0x12, 0x0c, 0x45,
		0xd2, 0x9a, 0xf6, 0x64, 0xe2, 0xff, 0x8f, 0xd4, 0x92, 0x18, 0xb7, 0x18,
		0xff, 0xff, 0xff, 0xff, 0x0f, 0x25, 0x50, 0xc3, 0x00, 0x00, 0x2a, 0x10,
		0x6d, 0x84, 0x67, 0x05, 0xba, 0x5c, 0x14, 0x3f, 0x94, 0x25, 0x72, 0x75,
		0xa2, 0xca, 0x70, 0x1f,
	}
	encodedPublicKey := []byte{
		0x04, 0x50, 0x44, 0x4f, 0xab, 0xe0, 0x62, 0xf6, 0xff, 0xc5, 0x9d, 0xe6,
		0x54, 0x37, 0x3e, 0x1a, 0xa8, 0x4a, 0xa4, 0xf0, 0x53, 0xf3, 0x65, 0xf3,
		0x74, 0x6f, 0xba, 0xaa, 0x8a, 0xd5, 0xd5, 0x87, 0x5e, 0x82, 0x79, 0xba,
		0x37, 0x9b, 0x47, 0x88, 0xe9, 0x14, 0x8f, 0x50, 0x00, 0xbd, 0x1f, 0xe0,
		0x85, 0xd1, 0x89, 0x25, 0xe8, 0xd0, 0x47, 0x23, 0x9c, 0xfa, 0x0d, 0x9c,
		0xd2, 0x34, 0x17, 0xb7, 0x14,
	}
	testEpoch := []byte{
		0xea, 0xab, 0xe3, 0x01, 0xb4, 0xb4, 0xa1, 0x24, 0x31, 0x18, 0xa4, 0x08,
		0x25, 0x22, 0x01, 0x15,
	}
	testVin := []byte("testvin")
	privateScalar := []byte{
		0x9d, 0x9c, 0x5e, 0x86, 0xd5, 0xa8, 0x30, 0x57, 0x96, 0x5c, 0xa9, 0x4b,
		0x5f, 0xae, 0xff, 0x11, 0x6c, 0x27, 0x5d, 0x08, 0xca, 0x91, 0xe8, 0x39,
		0xbd, 0x2d, 0xea, 0x59, 0xc2, 0x22, 0x9b, 0x81,
	}
	expectedPlaintext := []byte{
		0x82, 0x01, 0x4a, 0x2a, 0x48, 0x0a, 0x43, 0x0a, 0x41, 0x04, 0x4b, 0x96,
		0x80, 0x9e, 0x66, 0x82, 0x1b, 0x45, 0x53, 0xff, 0x2b, 0x42, 0x7a, 0x52,
		0xd5, 0x08, 0x43, 0xa2, 0xf7, 0x18, 0xfd, 0x56, 0xf2, 0xad, 0x93, 0xd4,
		0x6a, 0x8b, 0x1d, 0xb7, 0x4d, 0xd6, 0x19, 0x0f, 0x20, 0xb6, 0xb8, 0x2e,
		0x85, 0xa8, 0xea, 0xc4, 0x0e, 0x86, 0x7b, 0x9a, 0x0c, 0xf7, 0x4f, 0x36,
		0x4f, 0x03, 0xe9, 0x94, 0x6e, 0x25, 0xce, 0xee, 0x99, 0x34, 0xb1, 0x9c,
		0xb6, 0x03, 0x12, 0x01, 0x02, 0xca, 0x01, 0x09, 0x0a, 0x07, 0x74, 0x65,
		0x73, 0x74, 0x76, 0x69, 0x6e,
	}
	privateKey := UnmarshalECDHPrivateKey(privateScalar)
	if privateKey == nil {
		t.Fatalf("Failed to parse private key")
	}
	verifier, err := NewVerifier(privateKey, testVin, universal.Domain_DOMAIN_VEHICLE_SECURITY, encodedPublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if copy(verifier.epoch[:], testEpoch) != len(verifier.epoch) {
		t.Fatalf("Epoch length mismatch")
	}

	var message universal.RoutableMessage
	if err := proto.Unmarshal(encodedMessage, &message); err != nil {
		t.Fatalf("Couldn't decode message: %s", err)
	}
	plaintext, err := verifier.Verify(&message)
	if err != nil {
		t.Errorf("Failed to verify valid message: %s", err)
	} else if !bytes.Equal(expectedPlaintext, plaintext) {
		t.Errorf("Failed to recover original plaintext: %x", plaintext)
	}
}

func TestGCMNoCounterLongExpiration(t *testing.T) {
	verifier, signer := getGCMVerifierAndSigner(t)
	message := getTestMessage()
	if err := signer.encryptWithCounter(message, time.Hour, 0); err != nil {
		t.Fatalf("Error signing message: %s", err)
	}
	runVerifyTest(t, verifier, message, universal.MessageFault_E_MESSAGEFAULT_ERROR_TIME_TO_LIVE_TOO_LONG, true)
}

func TestGCMNoParameters(t *testing.T) {
	verifier, signer := getGCMVerifierAndSigner(t)
	message := getTestMessage()
	if err := signer.Encrypt(message, time.Minute); err != nil {
		t.Fatalf("Error signing message: %s", err)
	}
	message.GetSignatureData().SigType = nil
	runVerifyTest(t, verifier, message, errCodeBadParameter, false)
}

func TestGCMNonceLength(t *testing.T) {
	verifier, signer := getGCMVerifierAndSigner(t)
	message := getTestMessage()
	if err := signer.Encrypt(message, time.Minute); err != nil {
		t.Fatalf("Error signing message: %s", err)
	}
	sigData := message.GetSignatureData().GetAES_GCM_PersonalizedData()
	sigData.Nonce = append(sigData.Nonce, byte(3))
	runVerifyTest(t, verifier, message, universal.MessageFault_E_MESSAGEFAULT_ERROR_IV_INCORRECT_LENGTH, true)

	sigData.Nonce = []byte{3}
	runVerifyTest(t, verifier, message, universal.MessageFault_E_MESSAGEFAULT_ERROR_IV_INCORRECT_LENGTH, true)

	sigData.Nonce = nil
	runVerifyTest(t, verifier, message, universal.MessageFault_E_MESSAGEFAULT_ERROR_IV_INCORRECT_LENGTH, true)
}

func TestGCMWindow(t *testing.T) {
	var messages [windowSize]*universal.RoutableMessage
	duration := (maxSecondsWithoutCounter - 2) * time.Second
	verifier, signer := getGCMVerifierAndSigner(t)

	for k := 0; k < 3*windowSize; k++ {
		// Prepare messages with counter values 0 through windowSize - 1, but
		// don't send them.
		for i := 0; i < windowSize; i++ {
			messages[i] = getTestMessage()
			if err := signer.Encrypt(messages[i], duration); err != nil {
				t.Fatalf("Couldn't sign message: %s", err)
			}
		}

		// Prepare and send messages with counter values windowSize through
		// k + windowSize. Since the last counter value received will be C =
		// (k + windowSize), the window of acceptable counters will be [C -
		// windowSize, C) = [k, k + windowSize).
		for i := 0; i <= k; i++ {
			message := getTestMessage()
			if err := signer.Encrypt(message, duration); err != nil {
				t.Fatalf("Error signing message: %s", err)
			}
			runVerifyTest(t, verifier, message, errCodeOk, false)
		}

		for i := 0; i < windowSize; i++ {
			// Will loop through all our previously prepared  messages in a
			// "random" order since 97 is relatively prime to windowSize:
			j := ((i + 1) * 97) % windowSize
			// Since the window interval is now [k, k + windowSize), message j
			// should be accepted if and only if j >= k.
			if j >= k {
				runVerifyTest(t, verifier, messages[j], errCodeOk, false)
			}
			// Repeated message should fail, as should message that fall
			// outside the window.
			runVerifyTest(t, verifier, messages[j], errCodeInvalidToken, true)
		}
	}
}

func TestGCMWindowRecent(t *testing.T) {
	duration := (maxSecondsWithoutCounter - 2) * time.Second
	verifier, signer := getGCMVerifierAndSigner(t)
	var messages [windowSize * 2]*universal.RoutableMessage
	for i := 0; i < 2*windowSize; i++ {
		messages[i] = getTestMessage()
		if err := signer.Encrypt(messages[i], duration); err != nil {
			t.Fatalf("Couldn't sign message: %s", err)
		}
	}
	t.Logf("Window: %b, max: %d", verifier.window, verifier.counter)
	runVerifyTest(t, verifier, messages[2], errCodeOk, false)
	t.Logf("Window: %b, max: %d", verifier.window, verifier.counter)
	runVerifyTest(t, verifier, messages[3], errCodeOk, false)
	t.Logf("Window: %b, max: %d", verifier.window, verifier.counter)
	runVerifyTest(t, verifier, messages[4], errCodeOk, false)
	t.Logf("Window: %b, max: %d", verifier.window, verifier.counter)
	runVerifyTest(t, verifier, messages[8], errCodeOk, false)
	t.Logf("Window: %b, max: %d", verifier.window, verifier.counter)
	runVerifyTest(t, verifier, messages[windowSize], errCodeOk, false)
	t.Logf("Window: %b, max: %d", verifier.window, verifier.counter)
	runVerifyTest(t, verifier, messages[2*windowSize-1], errCodeOk, false)
	t.Logf("Window: %b, max: %d", verifier.window, verifier.counter)
	runVerifyTest(t, verifier, messages[windowSize], universal.MessageFault_E_MESSAGEFAULT_ERROR_INVALID_TOKEN_OR_COUNTER, true)
	t.Logf("Window: %b, max: %d", verifier.window, verifier.counter)
}

func TestHMAC(t *testing.T) {
	verifier, signer := getHMACVerifierAndSigner(t)
	message := getTestMessage()

	if err := signer.AuthorizeHMAC(message, time.Minute); err != nil {
		t.Fatalf("Error authorizing message: %s", err)
	}

	plaintext, err := verifier.Verify(message)
	if err != nil {
		t.Fatalf("Error verifying message: %s", err)
	}

	if !bytes.Equal(plaintext, testMessagePlaintext) {
		t.Errorf("Didn't recover plaintext")
	}
}

func TestHMACTampered(t *testing.T) {
	verifier, signer := getHMACVerifierAndSigner(t)
	message := getTestMessage()

	if err := signer.AuthorizeHMAC(message, time.Minute); err != nil {
		t.Fatalf("Error authorizing message: %s", err)
	}

	tag := message.GetSignatureData().GetHMAC_PersonalizedData().GetTag()
	if tag == nil {
		t.Fatalf("Tag not present")
	}
	tag[0] ^= 1

	runVerifyTest(t, verifier, message, errCodeInvalidSignature, true)

	tag[0] ^= 1
	runVerifyTest(t, verifier, message, errCodeOk, false)
}

func TestDetachedEncryptionUprev(t *testing.T) {
	verifier, signer := getVerifierAndSigner(t, DefaultVerifierFlags)
	verifier.domain = universal.Domain_DOMAIN_INFOTAINMENT
	message := getTestMessage()
	message.ToDestination = &universal.Destination{
		SubDestination: &universal.Destination_Domain{Domain: universal.Domain_DOMAIN_INFOTAINMENT},
	}
	action := carserver.Action{
		ActionMsg: &carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_VehicleControlSetValetModeAction{
					VehicleControlSetValetModeAction: &carserver.VehicleControlSetValetModeAction{
						Password: "1234",
					},
				},
			},
		},
	}
	plaintext, err := proto.Marshal(&action)
	if err != nil {
		t.Fatalf("Error creating command: %s", err)
	}
	message.Payload = &universal.RoutableMessage_ProtobufMessageAsBytes{
		ProtobufMessageAsBytes: plaintext,
	}
	// Expected behavior: the signer should use the verifier's session info, see that the verifier
	// supports detached AES-GCM, and automatically upgrade to that.
	if err := signer.AuthorizeHMAC(message, time.Minute); err != nil {
		t.Fatalf("Error signing message: %s", err)
	}
	if message.GetSignatureData().GetAES_GCM_DetachedData() == nil {
		t.Fatalf("Message not signed with detached GCM")
	}

	// The prefix should be a plaintext protobuf that indicates a Set Valet Mode action, but with
	// the message contents removed.
	if err := proto.Unmarshal(message.GetSignatureData().GetAES_GCM_DetachedData().GetPrefix(), &action); err != nil {
		t.Fatalf("Error decoding message: %s", err)
	}
	details := action.GetVehicleAction().GetVehicleControlSetValetModeAction()
	if details == nil {
		t.Fatalf("Wrong action type: %x", message.GetProtobufMessageAsBytes())
	}
	if details.GetPassword() != "" {
		t.Fatal("Failed to redact command parameters from plaintext data")
	}

	plaintext, err = verifier.Verify(message)
	if err != nil {
		t.Fatalf("Error verifying message: %s", err)
	}

	if err := proto.Unmarshal(plaintext, &action); err != nil {
		t.Fatalf("Error decoding message: %s", err)
	}

	if action.GetVehicleAction().GetVehicleControlSetValetModeAction().GetPassword() != "1234" {
		t.Fatal("Failed to recover original command parameters")
	}
}

func TestNoSignatureData(t *testing.T) {
	verifier, _ := getGCMVerifierAndSigner(t)
	message := getTestMessage()
	runVerifyTest(t, verifier, message, errCodeBadParameter, false)
}

func TestProvideHandle(t *testing.T) {
	verifier, _ := getGCMVerifierAndSigner(t)
	handle := uint32(0xDEADBEEF)
	verifier.AssignHandle(handle)
	info, err := verifier.SessionInfo()
	if err != nil {
		t.Fatal(err)
	}
	if info.GetHandle() != handle {
		t.Fatalf("Invalid handle value: %s", info)
	}
}

func TestVerifierEncryption(t *testing.T) {
	verifier, signer := getGCMVerifierAndSigner(t)
	message := getTestMessage()
	message.FromDestination = &universal.Destination{
		SubDestination: &universal.Destination_Domain{Domain: testVerifierDomain},
	}
	plaintext := append([]byte{}, message.GetProtobufMessageAsBytes()...)
	id := []byte{1, 2, 3}
	counter := uint32(4)
	t.Logf("Original message: %+v", message)
	if err := verifier.Encrypt(message, id, counter); err != nil {
		t.Fatalf("Error encrypting message: %s", err)
	}
	t.Logf("Encrypted message: %+v", message)
	if bytes.Equal(plaintext, message.GetProtobufMessageAsBytes()) {
		t.Fatalf("Encryption didn't change payload")
	}

	id[0] ^= 1
	if _, err := signer.Decrypt(message, id); err == nil {
		t.Fatalf("Decryption should have failed")
	}
	id[0] ^= 1
	if rxCounter, err := signer.Decrypt(message, id); err != nil || rxCounter != counter {
		t.Fatalf("Decryption failed: %s", err)
	}
	if !bytes.Equal(plaintext, message.GetProtobufMessageAsBytes()) {
		t.Fatalf("Decryption failed to recover original plaintext")
	}
}

func TestSignerDecryptNonceLength(t *testing.T) {
	verifier, signer := getGCMVerifierAndSigner(t)
	message := getTestMessage()
	message.FromDestination = &universal.Destination{
		SubDestination: &universal.Destination_Domain{Domain: testVerifierDomain},
	}
	id := []byte{1, 2, 3}
	if err := verifier.Encrypt(message, id, 4); err != nil {
		t.Fatalf("Error encrypting message: %s", err)
	}
	sigData := message.GetSignatureData().GetAES_GCM_ResponseData()
	for _, nonce := range [][]byte{append(sigData.Nonce, 3), {3}, nil} {
		sigData.Nonce = nonce
		ciphertext := append([]byte{}, message.GetProtobufMessageAsBytes()...)
		_, err := signer.Decrypt(message, id)
		if nativeErr, ok := err.(*Error); !ok || nativeErr.Code != universal.MessageFault_E_MESSAGEFAULT_ERROR_IV_INCORRECT_LENGTH {
			t.Errorf("Expected IV_INCORRECT_LENGTH but got %v", err)
		}
		if !bytes.Equal(ciphertext, message.GetProtobufMessageAsBytes()) {
			t.Errorf("Payload modified after decryption failure")
		}
	}
}

func TestSignerDecryptMetadataError(t *testing.T) {
	verifier, signer := getGCMVerifierAndSigner(t)
	message := getTestMessage()
	message.FromDestination = &universal.Destination{
		SubDestination: &universal.Destination_Domain{Domain: testVerifierDomain},
	}
	id := []byte{1, 2, 3}
	if err := verifier.Encrypt(message, id, 4); err != nil {
		t.Fatalf("Error encrypting message: %s", err)
	}
	ciphertext := append([]byte{}, message.GetProtobufMessageAsBytes()...)
	signer.verifierName = bytes.Repeat([]byte{'x'}, 256)
	if _, err := signer.Decrypt(message, id); !errors.Is(err, ErrMetadataFieldTooLong) {
		t.Errorf("Expected ErrMetadataFieldTooLong but got %v", err)
	}
	if !bytes.Equal(ciphertext, message.GetProtobufMessageAsBytes()) {
		t.Errorf("Payload modified after decryption failure")
	}
}

func vehicleActionMessage(t *testing.T, action *carserver.VehicleAction) *universal.RoutableMessage {
	t.Helper()
	plaintext, err := proto.Marshal(&carserver.Action{
		ActionMsg: &carserver.Action_VehicleAction{VehicleAction: action},
	})
	if err != nil {
		t.Fatalf("Error creating command: %s", err)
	}
	return &universal.RoutableMessage{
		ToDestination: &universal.Destination{
			SubDestination: &universal.Destination_Domain{Domain: universal.Domain_DOMAIN_INFOTAINMENT},
		},
		Payload: &universal.RoutableMessage_ProtobufMessageAsBytes{ProtobufMessageAsBytes: plaintext},
	}
}

// Detached encryption would hide the operation of commands whose only parameter is an empty option
// that selects it, such as charge start and stop, so AuthorizeHMAC keeps the HMAC tag for them.
func TestAuthorizeHMACKeepsOperationInPlaintext(t *testing.T) {
	startStop := func(action *carserver.ChargingStartStopAction) *carserver.VehicleAction {
		return &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_ChargingStartStopAction{ChargingStartStopAction: action},
		}
	}
	tests := map[string]struct {
		action   *carserver.VehicleAction
		detached bool
	}{
		"charge start": {startStop(&carserver.ChargingStartStopAction{
			ChargingAction: &carserver.ChargingStartStopAction_Start{Start: &carserver.Void{}},
		}), false},
		"charge stop": {startStop(&carserver.ChargingStartStopAction{
			ChargingAction: &carserver.ChargingStartStopAction_Stop{Stop: &carserver.Void{}},
		}), false},
		"charge start max range": {startStop(&carserver.ChargingStartStopAction{
			ChargingAction: &carserver.ChargingStartStopAction_StartMaxRange{StartMaxRange: &carserver.Void{}},
		}), false},
		"vent windows": {&carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_VehicleControlWindowAction{
				VehicleControlWindowAction: &carserver.VehicleControlWindowAction{
					Action: &carserver.VehicleControlWindowAction_Vent{Vent: &carserver.Void{}},
				},
			},
		}, false},
		"set charging amps": {&carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_SetChargingAmpsAction{
				SetChargingAmpsAction: &carserver.SetChargingAmpsAction{ChargingAmps: 16},
			},
		}, true},
		"open charge port (no parameters)": {&carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_ChargePortDoorOpen{ChargePortDoorOpen: &carserver.ChargePortDoorOpen{}},
		}, true},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			verifier, signer := getVerifierAndSigner(t, DefaultVerifierFlags)
			verifier.domain = universal.Domain_DOMAIN_INFOTAINMENT
			message := vehicleActionMessage(t, test.action)
			original := append([]byte{}, message.GetProtobufMessageAsBytes()...)

			if err := signer.AuthorizeHMAC(message, time.Minute); err != nil {
				t.Fatalf("Error signing message: %s", err)
			}
			if got := message.GetSignatureData().GetAES_GCM_DetachedData() != nil; got != test.detached {
				t.Fatalf("detached=%v, want %v", got, test.detached)
			}
			if !test.detached {
				if message.GetSignatureData().GetHMAC_PersonalizedData() == nil {
					t.Fatal("Message not signed with HMAC")
				}
				if !bytes.Equal(message.GetProtobufMessageAsBytes(), original) {
					t.Errorf("HMAC-signed command was modified: %x", message.GetProtobufMessageAsBytes())
				}
			}
			plaintext, err := verifier.Verify(message)
			if err != nil {
				t.Fatalf("Error verifying message: %s", err)
			}
			if !bytes.Equal(plaintext, original) {
				t.Errorf("Recovered %x instead of %x", plaintext, original)
			}
		})
	}
}

func TestDetachedPayloadIsOperation(t *testing.T) {
	tests := map[string]struct {
		domain  universal.Domain
		payload []byte
		want    bool
	}{
		// 2: { 6: { 5: {} }}, charge stop
		"one empty option": {universal.Domain_DOMAIN_INFOTAINMENT, []byte{0x12, 0x04, 0x32, 0x02, 0x2a, 0x00}, true},
		// 2: { 6: { 2: {}, 5: {} }}
		"several empty options": {universal.Domain_DOMAIN_INFOTAINMENT, []byte{0x12, 0x06, 0x32, 0x04, 0x12, 0x00, 0x2a, 0x00}, true},
		// 2: { 6: {} }
		"no parameters": {universal.Domain_DOMAIN_INFOTAINMENT, []byte{0x12, 0x02, 0x32, 0x00}, false},
		// 2: { 91: { 1: 16 }}, set charging amps
		"varint parameter": {universal.Domain_DOMAIN_INFOTAINMENT, []byte{0x12, 0x05, 0xda, 0x05, 0x02, 0x08, 0x10}, false},
		// 2: { 46: { 1: "secret" }}
		"string parameter": {universal.Domain_DOMAIN_INFOTAINMENT, []byte{0x12, 0x0a, 0xf2, 0x02, 0x07, 0x08, 0x73, 0x65, 0x63, 0x72, 0x65, 0x74}, false},
		// 2: { 6: { 5: {}, 1: 1 }}
		"empty option beside a parameter": {universal.Domain_DOMAIN_INFOTAINMENT, []byte{0x12, 0x06, 0x32, 0x04, 0x2a, 0x00, 0x08, 0x01}, false},
		// 2: { 6: { 5: { 1: 1 } }}
		"non-empty option": {universal.Domain_DOMAIN_INFOTAINMENT, []byte{0x12, 0x06, 0x32, 0x04, 0x2a, 0x02, 0x08, 0x01}, false},
		// 2: { 6: { 5: <truncated> }}
		"truncated option":   {universal.Domain_DOMAIN_INFOTAINMENT, []byte{0x12, 0x04, 0x32, 0x02, 0x2a, 0x05}, false},
		"invalid command":    {universal.Domain_DOMAIN_INFOTAINMENT, []byte{0x12, 0x00}, false},
		"unsupported domain": {universal.Domain_DOMAIN_BROADCAST, []byte{0x12, 0x04, 0x32, 0x02, 0x2a, 0x00}, false},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			message := &universal.RoutableMessage{
				ToDestination: &universal.Destination{
					SubDestination: &universal.Destination_Domain{Domain: test.domain},
				},
				Payload: &universal.RoutableMessage_ProtobufMessageAsBytes{ProtobufMessageAsBytes: test.payload},
			}
			if got := detachedPayloadIsOperation(message); got != test.want {
				t.Errorf("detachedPayloadIsOperation(%x) = %v, want %v", test.payload, got, test.want)
			}
		})
	}
}
