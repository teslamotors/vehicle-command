package authentication

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"time"

	universal "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/universalmessage"
)

func ExampleDispatcher() {
	const carCount = 10
	const messagesPerCar = 5
	var challenges [][]byte

	// Create dispatcher
	dispatcherPrivateKey, err := NewECDHPrivateKey(rand.Reader)
	if err != nil {
		panic(fmt.Sprintf("Failed to generate dispatcher key: %s", err))
	}
	dispatcher := Dispatcher{dispatcherPrivateKey}
	// Initialize cars, provision dispatcher public key
	cars := make([]*Verifier, carCount)
	for i := 0; i < carCount; i++ {
		vin := []byte(fmt.Sprintf("%d", i))
		VCSECKey, err := NewECDHPrivateKey(rand.Reader)
		var challenge [16]byte
		if _, err := rand.Read(challenge[:]); err != nil {
			panic(fmt.Sprintf("Failed to generate random challenge: %s", err))
		} else {
			challenges = append(challenges, challenge[:])
		}
		if err != nil {
			panic(fmt.Sprintf("Failed to generate car key: %s", err))
		}
		cars[i], err = NewVerifier(VCSECKey,
			vin, universal.Domain_DOMAIN_VEHICLE_SECURITY,
			dispatcherPrivateKey.PublicBytes())
		if err != nil {
			panic(fmt.Sprintf("Failed to initialize car: %s", err))
		}
	}

	message := &universal.RoutableMessage{
		ToDestination: &universal.Destination{
			SubDestination: &universal.Destination_Domain{Domain: universal.Domain_DOMAIN_VEHICLE_SECURITY},
		},
	}
	for i := 0; i < carCount; i++ {
		// Fetch session info from vehicle
		sessionInfo, tag, err := cars[i].SignedSessionInfo(challenges[i])
		if err != nil {
			panic(fmt.Sprintf("Error obtaining session info from car %d: %s", i, err))
		}
		// Give it to the signer (dispatcher). The UUID used to fetch the
		// session info can be used as the challenge.
		connection, err := dispatcher.ConnectAuthenticated(cars[i].verifierName, challenges[i], sessionInfo, tag)
		if err != nil {
			panic(fmt.Sprintf("Error creating authenticated connection to car %d: %s", i, err))
		}

		// Send several messages to vehicle using connection
		for j := 0; j < messagesPerCar; j++ {
			original := []byte(fmt.Sprintf("Message %d for car %d", j, i))
			message.Payload = &universal.RoutableMessage_ProtobufMessageAsBytes{ProtobufMessageAsBytes: original}
			if err := connection.Encrypt(message, time.Minute); err != nil {
				panic(fmt.Sprintf("Failed to encrypt message: %s", err))
			}

			// This won't happen if err above is nil, just here for illustrative purposes.
			if bytes.Equal(message.GetProtobufMessageAsBytes(), original) {
				panic("Message wasn't encrypted!")
			}

			if plaintext, err := cars[i].Verify(message); err != nil {
				panic(fmt.Sprintf("Decryption error :%s", err))
			} else if !bytes.Equal(plaintext, original) {
				panic("Failed to recover original plaintext")
			}
		}
	}
}
