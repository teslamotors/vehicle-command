package authentication

import (
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	universal "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/universalmessage"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/vcsec"
)

func newSignerForDomain(t *testing.T, domain universal.Domain) (*Verifier, *Signer) {
	t.Helper()
	verifierKey, signerKey := getVerifierAndSignerKeys(t)
	name := []byte("test_verifier")
	challenge := []byte{0, 1, 2, 3, 4, 5, 6, 7}
	verifier, err := NewVerifier(verifierKey, name, domain, signerKey.PublicBytes())
	if err != nil {
		t.Fatal(err)
	}
	info, tag, err := verifier.SignedSessionInfo(challenge)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := NewAuthenticatedSigner(signerKey, name, challenge, info, tag)
	if err != nil {
		t.Fatal(err)
	}
	return verifier, signer
}

func TestNewVerifierDetachedFlagByDomain(t *testing.T) {
	tests := map[universal.Domain]bool{
		universal.Domain_DOMAIN_INFOTAINMENT:     true,
		universal.Domain_DOMAIN_VEHICLE_SECURITY: false,
	}
	for domain, want := range tests {
		t.Run(domain.String(), func(t *testing.T) {
			_, signer := newSignerForDomain(t, domain)
			if signer.SupportsDetached != want {
				t.Errorf("SupportsDetached = %v, want %v", signer.SupportsDetached, want)
			}
		})
	}
}

func TestAuthorizeHMACVehicleSecurityRKEAction(t *testing.T) {
	verifier, signer := newSignerForDomain(t, universal.Domain_DOMAIN_VEHICLE_SECURITY)
	payload, err := proto.Marshal(&vcsec.UnsignedMessage{
		SubMessage: &vcsec.UnsignedMessage_RKEAction{RKEAction: vcsec.RKEAction_E_RKE_ACTION_LOCK},
	})
	if err != nil {
		t.Fatal(err)
	}
	message := &universal.RoutableMessage{
		ToDestination: &universal.Destination{
			SubDestination: &universal.Destination_Domain{Domain: universal.Domain_DOMAIN_VEHICLE_SECURITY},
		},
		Payload: &universal.RoutableMessage_ProtobufMessageAsBytes{ProtobufMessageAsBytes: payload},
	}
	if err := signer.AuthorizeHMAC(message, time.Minute); err != nil {
		t.Fatalf("AuthorizeHMAC: %s", err)
	}
	if message.GetSignatureData().GetHMAC_PersonalizedData() == nil {
		t.Fatal("Expected HMAC signature data")
	}
	if _, err := verifier.Verify(message); err != nil {
		t.Fatalf("Verify: %s", err)
	}
}
