package authentication

import (
	"testing"

	universal "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/universalmessage"
)

const (
	testVerifierDomain = universal.Domain_DOMAIN_VEHICLE_SECURITY
)

var (
	testMessagePlaintext = []byte("hello world")
)

func getVerifierAndSignerKeys(t *testing.T) (ECDHPrivateKey, ECDHPrivateKey) {
	t.Helper()
	// Generate a private key, extract the private scalar, convert to comma-separated hex:
	// openssl ecparam -genkey -noout -name prime256v1 | openssl asn1parse | grep "OCTET STRING" | cut -f4 -d: | xxd -r -p | xxd -i
	verifierScalar := []byte{
		0x72, 0xe9, 0xa4, 0x93, 0xba, 0x41, 0xe7, 0x92, 0xb1, 0x04, 0x27, 0x43,
		0x31, 0x10, 0x5f, 0xa6, 0xc9, 0x08, 0xc2, 0x7f, 0x15, 0x91, 0x3e, 0xec,
		0xc2, 0xf4, 0xec, 0x11, 0x5b, 0x28, 0x1a, 0xe0,
	}
	signerScalar := []byte{
		0x48, 0x07, 0xe2, 0x9d, 0x46, 0x42, 0x5d, 0x07, 0xdf, 0x48, 0x19, 0x32,
		0x49, 0xa6, 0x24, 0x1d, 0x41, 0x1a, 0xc4, 0x00, 0x73, 0x75, 0xc7, 0x5d,
		0x5d, 0x4a, 0x22, 0xec, 0xf1, 0x89, 0xcd, 0xde,
	}
	verifierPrivateKey := UnmarshalECDHPrivateKey(verifierScalar)
	signerPrivateKey := UnmarshalECDHPrivateKey(signerScalar)
	return verifierPrivateKey, signerPrivateKey
}

func getGCMVerifierAndSigner(t *testing.T) (*Verifier, *Signer) {
	t.Helper()
	verifierPrivateKey, signerPrivateKey := getVerifierAndSignerKeys(t)
	challenge := []byte{0, 1, 2, 3, 4, 5, 6, 7}
	verifierName := []byte("test_verifier")

	verifier, err := NewVerifier(verifierPrivateKey, verifierName, testVerifierDomain, signerPrivateKey.PublicBytes())
	if err != nil {
		t.Fatalf("Couldn't initialize Verifier: %s", err)
	}
	verifierInfo, tag, err := verifier.SignedSessionInfo(challenge)
	if err != nil {
		t.Fatalf("Couldn't fetch verifier session info: %s", err)
	}
	signer, err := NewAuthenticatedSigner(signerPrivateKey, verifierName, challenge, verifierInfo, tag)
	if err != nil {
		t.Fatalf("Couldn't generate signer: %s", err)
	}
	return verifier, signer
}

func getTestMessage() *universal.RoutableMessage {
	payload := make([]byte, len(testMessagePlaintext))
	copy(payload, testMessagePlaintext)
	message := &universal.RoutableMessage{
		ToDestination: &universal.Destination{
			SubDestination: &universal.Destination_Domain{Domain: testVerifierDomain},
		},
		Payload: &universal.RoutableMessage_ProtobufMessageAsBytes{
			ProtobufMessageAsBytes: payload,
		},
	}
	return message
}

func checkError(t *testing.T, err error, expectedCode universal.MessageFault_E) {
	t.Helper()
	if expectedCode == errCodeOk {
		if err == nil {
			return
		}
		t.Errorf("Expected success but got %s", err)
	}
	if comErr, ok := err.(*Error); ok {
		if comErr.Code != expectedCode {
			t.Errorf("Expected %s but got %s", expectedCode, comErr.Code)
		}
	} else {
		t.Errorf("Got unexpected error type: %s", err)
	}
}
