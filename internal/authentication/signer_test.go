package authentication

import (
	"bytes"
	"crypto/rand"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	universal "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/universalmessage"
)

func TestUpdateSessionInfo(t *testing.T) {
	verifier, signer := getGCMVerifierAndSigner(t)
	challenge := []byte{0, 1, 2, 3, 4, 5, 6, 7}
	info, tag, err := verifier.SignedSessionInfo(challenge)
	if err != nil {
		t.Fatalf("Failed to generate session info: %s", err)
	}
	if err = signer.UpdateSignedSessionInfo(challenge, info, tag); err != nil {
		t.Fatalf("Rejected valid session info update: %s", err)
	}
}

func TestBadSessionInfoProto(t *testing.T) {
	verifierId := []byte("foo")
	verifierKey, signerKey := getVerifierAndSignerKeys(t)
	dispatcher := Dispatcher{signerKey}

	verifier, err := NewVerifier(verifierKey, verifierId, universal.Domain_DOMAIN_VEHICLE_SECURITY, signerKey.PublicBytes())
	if err != nil {
		t.Fatalf("Failed to generate verifier")
	}

	challenge := []byte{0, 1, 2, 3, 4, 5, 6, 7}
	info, tag, err := verifier.SignedSessionInfo(challenge)
	if err != nil {
		t.Fatalf("Failed to get session info: %s", err)
	}

	signer, err := dispatcher.ConnectAuthenticated(verifierId, challenge, info, tag)
	if err != nil {
		t.Errorf("Error connecting to verifier: %s", err)
	}

	info[0] ^= 1
	err = signer.UpdateSignedSessionInfo(challenge, info, tag)
	checkError(t, err, errCodeInvalidSignature)

	_, err = dispatcher.ConnectAuthenticated(verifierId, challenge, info, tag)
	checkError(t, err, errCodeDecoding)

	if tag, err = verifier.session.SessionInfoHMAC(verifier.verifierName, challenge, info); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	err = signer.UpdateSignedSessionInfo(challenge, info, tag)
	checkError(t, err, errCodeDecoding)
}

func TestBadSessionInfoTag(t *testing.T) {
	verifier, signer := getGCMVerifierAndSigner(t)
	challenge := []byte{0, 1, 2, 3, 4, 5, 6, 7}
	info, tag, err := verifier.SignedSessionInfo(challenge)
	if err != nil {
		t.Fatalf("Failed to get session info: %s", err)
	}
	tag[0] ^= 1
	err = signer.UpdateSignedSessionInfo(challenge, info, tag)
	checkError(t, err, errCodeInvalidSignature)
}

func TestUpdateSessionInfoBadChallenge(t *testing.T) {
	verifier, signer := getGCMVerifierAndSigner(t)
	challenge := []byte{0, 1, 2, 3, 4, 5, 6, 7}
	info, tag, err := verifier.SignedSessionInfo(challenge)
	if err != nil {
		t.Fatalf("Failed to get session info: %s", err)
	}
	challenge[0] ^= 1
	err = signer.UpdateSignedSessionInfo(challenge, info, tag)
	checkError(t, err, errCodeInvalidSignature)
}

func TestUpdateSessionInfoBadCounter(t *testing.T) {
	verifier, signer := getGCMVerifierAndSigner(t)
	challenge := []byte{0, 1, 2, 3, 4, 5, 6, 7}
	info, tag, err := verifier.SignedSessionInfo(challenge)
	if err != nil {
		t.Fatalf("Failed to get session info: %s", err)
	}
	decoded := decodeInfo(info)
	decoded.Counter++
	info, err = proto.Marshal(decoded)
	if err != nil {
		t.Fatal("Error re-encoding data")
	}
	err = signer.UpdateSignedSessionInfo(challenge, info, tag)
	checkError(t, err, errCodeInvalidSignature)
}

func TestUpdateSessionInfoBadEpoch(t *testing.T) {
	verifier, signer := getGCMVerifierAndSigner(t)
	challenge := []byte{0, 1, 2, 3, 4, 5, 6, 7}
	info, tag, err := verifier.SignedSessionInfo(challenge)
	if err != nil {
		t.Fatalf("Failed to get session info: %s", err)
	}
	decoded := decodeInfo(info)
	decoded.Epoch[0] ^= 0x01
	info, err = proto.Marshal(decoded)
	if err != nil {
		t.Fatal("Error re-encoding data")
	}
	err = signer.UpdateSignedSessionInfo(challenge, info, tag)
	checkError(t, err, errCodeInvalidSignature)
}

func TestUpdateSessionInfoBadPublicKey(t *testing.T) {
	verifier, signer := getGCMVerifierAndSigner(t)
	challenge := []byte{0, 1, 2, 3, 4, 5, 6, 7}
	imposterKey, err := NewECDHPrivateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	imposter, err := NewVerifier(
		imposterKey, verifier.verifierName, testVerifierDomain,
		signer.session.LocalPublicBytes())
	if err != nil {
		t.Fatalf("Failed to initialize verifier: %s", err)
	}
	info, tag, err := imposter.SignedSessionInfo(challenge)
	if err != nil {
		t.Fatalf("Failed to generate session info: %s", err)
	}
	err = signer.UpdateSignedSessionInfo(challenge, info, tag)
	checkError(t, err, errCodeInvalidSignature)

	if tag, err = verifier.session.SessionInfoHMAC(verifier.verifierName, challenge, info); err != nil {
		t.Fatalf("Unexpected error %s", err)
	}
	err = signer.UpdateSignedSessionInfo(challenge, info, tag)
	checkError(t, err, errCodeUnknownKey)
}

func TestRemotePublicKey(t *testing.T) {
	verifier, signer := getGCMVerifierAndSigner(t)
	info, err := verifier.SessionInfo()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(info.GetPublicKey(), signer.RemotePublicKeyBytes()) {
		t.Errorf("Mismatched RemotePublicKeyBytes")
	}
}

func TestUpdateInvalidSessionInfo(t *testing.T) {
	challenge := []byte("challenge")
	verifier, signer := getGCMVerifierAndSigner(t)

	// Verify nominal path works
	info, tag, err := verifier.SignedSessionInfo(challenge)
	if err != nil {
		t.Fatalf("Couldn't get session info: %s", err)
	}
	message := getTestMessage()
	if err := signer.Encrypt(message, time.Second); err != nil {
		t.Fatalf("Error signing message: %s", err)
	}
	runVerifyTest(t, verifier, message, errCodeOk, false)

	// Authorize a second command. We'll simulate this arriving out-of-order
	// with a now stale session info update.
	message2 := getTestMessage()
	if err := signer.Encrypt(message2, time.Second); err != nil {
		t.Fatalf("Error signing message: %s", err)
	}

	// "Update" to the stale session info and verify the first message is still
	// rejected; that is, verify that the verifier didn't roll back its counter
	// based on the stale session info.
	if err := signer.UpdateSignedSessionInfo(challenge, info, tag); err != nil {
		t.Fatalf("Rejected valid session info: %s", err)
	}
	runVerifyTest(t, verifier, message, errCodeInvalidToken, true)
	// The second message should still work
	runVerifyTest(t, verifier, message2, errCodeOk, false)
}

func TestSignerCounterRollover(t *testing.T) {
	verifier, signer := getGCMVerifierAndSigner(t)
	info, err := verifier.SessionInfo()
	if err != nil {
		t.Fatalf("Error creating session info: %s", err)
	}
	info.Counter = 0xFFFFFFFF
	err = signer.UpdateSessionInfo(info)
	if err != nil {
		t.Fatalf("Failed to update session info: %s", err)
	}
	message := getTestMessage()
	err = signer.Encrypt(message, time.Second)
	checkError(t, err, errCodeInvalidToken)

	// Check signer gets locked in this state
	message = getTestMessage()
	err = signer.Encrypt(message, time.Second)
	checkError(t, err, errCodeInvalidToken)
}

func TestNewAuthenticatedSigner(t *testing.T) {
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

	info[0] ^= 1
	_, err = NewAuthenticatedSigner(signerKey, id, challenge, info, tag)
	checkError(t, err, errCodeDecoding)
	info[0] ^= 1

	tag[0] ^= 1
	_, err = NewAuthenticatedSigner(signerKey, id, challenge, info, tag)
	checkError(t, err, errCodeInvalidSignature)
}

func TestSetSessionInfo(t *testing.T) {
	verifier, signer := getGCMVerifierAndSigner(t)
	challenge := []byte{1, 2, 3, 4, 5}
	message := getTestMessage()
	if err := verifier.SetSessionInfo(challenge, message); err != nil {
		t.Fatal(err)
	}
	if err := signer.UpdateSignedSessionInfo(
		challenge,
		message.GetSessionInfo(),
		message.GetSignatureData().GetSessionInfoTag().Tag,
	); err != nil {
		t.Error(err)
	}

	challenge[0] ^= 1
	if err := signer.UpdateSignedSessionInfo(
		challenge,
		message.GetSessionInfo(),
		message.GetSignatureData().GetSessionInfoTag().Tag,
	); err == nil {
		t.Error(err)
	}
}

func TestExportImport(t *testing.T) {
	verifier, signer := getGCMVerifierAndSigner(t)
	_, skey := getVerifierAndSignerKeys(t)
	message := getTestMessage()
	if err := signer.Encrypt(message, time.Minute); err != nil {
		t.Fatalf("Error signing message: %s", err)
	}
	runVerifyTest(t, verifier, message, errCodeOk, false)

	cache, err := signer.ExportSessionInfo()
	if err != nil {
		t.Fatal(err)
	}

	elapsedTime := 30 * time.Minute
	verifier.timeZero = verifier.timeZero.Add(-elapsedTime)

	signer, err = ImportSessionInfo(skey, verifier.verifierName, cache, time.Now().Add(-elapsedTime))
	if err != nil {
		t.Fatal(err)
	}
	message = getTestMessage()
	if err := signer.Encrypt(message, time.Minute); err != nil {
		t.Fatalf("Error signing message: %s", err)
	}
	runVerifyTest(t, verifier, message, errCodeOk, false)
}

func TestImportWrongTime(t *testing.T) {
	verifier, signer := getGCMVerifierAndSigner(t)
	_, skey := getVerifierAndSignerKeys(t)
	message := getTestMessage()
	if err := signer.Encrypt(message, time.Minute); err != nil {
		t.Fatalf("Error signing message: %s", err)
	}
	runVerifyTest(t, verifier, message, errCodeOk, false)

	cache, err := signer.ExportSessionInfo()
	if err != nil {
		t.Fatal(err)
	}

	elapsedTime := 30 * time.Minute
	verifier.timeZero = verifier.timeZero.Add(-elapsedTime)

	signer, err = ImportSessionInfo(skey, verifier.verifierName, cache, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	message = getTestMessage()
	if err := signer.Encrypt(message, time.Minute); err != nil {
		t.Fatalf("Error signing message: %s", err)
	}
	runVerifyTest(t, verifier, message, errCodeExpired, true)
}

func TestInvalidExpirationTime(t *testing.T) {
	_, signer := getGCMVerifierAndSigner(t)
	message := getTestMessage()

	if err := signer.AuthorizeHMAC(message, 2*epochLength); err == nil {
		t.Fatal("Expected error when authorizing message with invalid expiration time")
	}
}
