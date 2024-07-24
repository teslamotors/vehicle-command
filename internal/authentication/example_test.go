package authentication_test

import (
	"crypto/rand"
	"fmt"
	"time"

	command "github.com/teslamotors/vehicle-command/internal/authentication"
	universal "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/universalmessage"
)

func Example() {
	/***** One-time setup ********************************************************/
	// Executed by Verifier: Generate key pair
	verifierKey, err := command.NewECDHPrivateKey(rand.Reader)
	if err != nil {
		panic(fmt.Sprintf("Failed to generate verifier key: %s", err))
	}

	// Executed by Signer: Generate key pair
	signerKey, err := command.NewECDHPrivateKey(rand.Reader)
	if err != nil {
		panic(fmt.Sprintf("Failed to generate signer key: %s", err))
	}

	// Signer and Verifier exchange public keys through an authenticated channel.
	// Signer must know Verifier domain and name (see protocol description).
	domain := universal.Domain_DOMAIN_VEHICLE_SECURITY
	verifierId := []byte("testVIN-1234")

	/***** Once per session *****************************************************/
	// (A session typically lasts until either Signer or Verifier reboots)

	verifier, err := command.NewVerifier(verifierKey, verifierId, domain, signerKey.PublicBytes())
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize verifier: %s", err))
	}

	// Signer sends GetSessionInfo request to Verifier, identifying itself by its public key.
	// Verifier uses the UUID from the request as the challenge value below.
	challenge := []byte{0, 1, 2, 3, 4, 5, 6, 7}
	encodedInfo, tag, err := verifier.SignedSessionInfo(challenge)
	if err != nil {
		panic(fmt.Sprintf("Failed to get session info: %s", err))
	}

	// Verifier sends encodedInfo, tag to Signer.
	// Signer executes:
	signer, err := command.NewAuthenticatedSigner(signerKey, verifierId, challenge, encodedInfo, tag)
	if err != nil {
		panic(fmt.Sprintf("Failed to initialize signer: %s", err))
	}

	/***** Once per message *****************************************************/
	// Signer constructs message to transmit:
	message := &universal.RoutableMessage{
		ToDestination: &universal.Destination{
			SubDestination: &universal.Destination_Domain{Domain: domain},
		},
	}
	message.Payload = &universal.RoutableMessage_ProtobufMessageAsBytes{
		ProtobufMessageAsBytes: []byte("hello world"),
	}
	// Encrypt message. Expires in one minute.
	if err := signer.Encrypt(message, time.Minute); err != nil {
		panic(fmt.Sprintf("Failed to encrypt message: %s", err))
	}

	// Signer marshals message, transmits to Verifier
	// Verifier executes:
	plaintext, err := verifier.Verify(message)
	if err != nil {
		panic(fmt.Sprintf("Failed to decrypt: %s", err))
	}
	fmt.Printf("%s\n", plaintext)

	// Second message can reuse existing session:
	message.Payload = &universal.RoutableMessage_ProtobufMessageAsBytes{
		ProtobufMessageAsBytes: []byte("Goodbye!"),
	}
	if err := signer.Encrypt(message, time.Minute); err != nil {
		panic(fmt.Sprintf("Failed to encrypt message: %s", err))
	}

	plaintext, err = verifier.Verify(message)
	if err != nil {
		panic(fmt.Sprintf("Failed to decrypt: %s", err))
	}
	fmt.Printf("%s\n", plaintext)

	// Output:
	// hello world
	// Goodbye!
}
