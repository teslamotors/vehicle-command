package authentication

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/signatures"
	universal "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/universalmessage"
)

// A Verifier checks the authenticity of commands sent by a Signer.
type Verifier struct {
	Peer
	lock   sync.Mutex
	window uint64
}

// NewVerifier returns a Verifier.
// Set domain to universal.Domain_DOMAIN_BROADCAST if the Verifier shouldn't enforce domain
// checking. The Verifier's domain must be known in advance by the Signer.
func NewVerifier(private ECDHPrivateKey, id []byte, domain universal.Domain, signerPublicBytes []byte) (*Verifier, error) {
	session, err := private.Exchange(signerPublicBytes)
	if err != nil {
		return nil, err
	}
	verifier := Verifier{
		Peer: Peer{
			domain:       domain,
			verifierName: id,
			session:      session,
		},
		window: 0,
	}

	if len(id) > 255 {
		return nil, ErrMetadataFieldTooLong
	}

	if err := verifier.rotateEpochIfNeeded(false); err != nil {
		return nil, err
	}
	return &verifier, nil
}

func (v *Verifier) signatureError(code universal.MessageFault_E, challenge []byte) error {
	if info, tag, err := v.signedSessionInfo(challenge); err == nil {
		return &InvalidSignatureError{
			Code:        code,
			EncodedInfo: info,
			Tag:         tag,
		}
	}
	return newError(errCodeInternal, fmt.Sprintf("Error collecting session info after encountering %s", code))
}

func (v *Verifier) rotateEpochIfNeeded(force bool) error {
	if force || v.timeZero.IsZero() || v.counter == 0xFFFFFFFF || v.timestamp() > uint32(epochLength/time.Second) {
		if _, err := rand.Read(v.epoch[:]); err != nil {
			v.counter = 0xFFFFFFFF
			return newError(errCodeInternal, "RNG failure")
		} else {
			v.timeZero = time.Now()
			v.counter = 0
		}
	}
	return nil
}

func (v *Verifier) sessionInfo() (*signatures.SessionInfo, error) {
	if err := v.adjustClock(); err != nil {
		return nil, err
	}
	info := &signatures.SessionInfo{
		Counter:   v.counter,
		PublicKey: v.session.LocalPublicBytes(),
		Epoch:     v.epoch[:],
		ClockTime: v.timestamp(),
	}
	return info, nil
}

// SessionInfo contains metadata used to prevent replay and similar attacks.
// A Signer must have the Verifier's SessionInfo on initialization.
func (v *Verifier) SessionInfo() (*signatures.SessionInfo, error) {
	v.lock.Lock()
	defer v.lock.Unlock()
	return v.sessionInfo()
}

func (v *Verifier) signedSessionInfo(challenge []byte) (encodedInfo, tag []byte, err error) {
	info, err := v.sessionInfo()
	if err != nil {
		return
	}
	encodedInfo, err = proto.Marshal(info)
	if err != nil {
		return
	}
	tag, err = v.session.SessionInfoHMAC(v.verifierName, challenge, encodedInfo)
	return
}

// SignedSessionInfo returns a protobuf-encoded signatures.SessionInfo along with an authentication
// tag that a Signer may use to verify the info has not been tampered with.
func (v *Verifier) SignedSessionInfo(challenge []byte) (encodedInfo, tag []byte, err error) {
	v.lock.Lock()
	defer v.lock.Unlock()
	return v.signedSessionInfo(challenge)
}

// SetSessionInfo attaches up-to-date session info to a message.
// This is useful when v encounters an error while authenticating a message and wishes to include
// session info in the error response, thereby allowing the Signer to resync.
func (v *Verifier) SetSessionInfo(challenge []byte, message *universal.RoutableMessage) error {
	encodedInfo, tag, err := v.SignedSessionInfo(challenge)
	if err != nil {
		return err
	}
	message.Payload = &universal.RoutableMessage_SessionInfo{
		SessionInfo: encodedInfo,
	}
	message.SubSigData = &universal.RoutableMessage_SignatureData{
		SignatureData: &signatures.SignatureData{
			SigType: &signatures.SignatureData_SessionInfoTag{
				SessionInfoTag: &signatures.HMAC_Signature_Data{
					Tag: tag,
				},
			},
		},
	}
	return nil
}

func (v *Verifier) adjustClock() error {
	// During process sleep, the monotonic clock may be frozen. This has the effect of causing
	// commands to expire further in the future then the client may intend. In order to correct, we
	// check if the wall clock has advanced significantly further than the monotonic clock and
	// update the monotonic clock accordingly.
	//
	// Since the wall clock can be set over UDP, it is not trustworthy; therefore we do not make
	// corrections in the other direction. This means an attacker who can modify the wall clock
	// can cause commands to expire prematurely, but cannot extend the expiration time of a command.
	//
	// See https://pkg.go.dev/time discussion on wall clocks vs monotonic clock.
	now := time.Now()
	wallClock := now.Unix()
	wallClockStart := v.timeZero.Unix()

	// Check values that would cause an overflow.
	const yearInSeconds = 365 * 24 * 60 * 60
	if wallClockStart > wallClock || wallClock-wallClockStart > yearInSeconds {
		return v.rotateEpochIfNeeded(true)
	}

	// elapsedWallTime and elapsedProcessTime are how far into the current epoch we are according to
	// the wall clock and the monotonic clock, respectively.
	elapsedWallTime := time.Duration(wallClock-wallClockStart) * time.Second
	elapsedProcessTime := now.Sub(v.timeZero) // Monotonic clock semantics promise this will not be negative, and the implementation uses saturated arithmetic.

	if elapsedWallTime > elapsedProcessTime {
		sleepDuration := elapsedWallTime - elapsedProcessTime
		if sleepDuration > time.Second {
			var t time.Time
			if t.Add(elapsedWallTime).After(now) {
				return v.rotateEpochIfNeeded(true)
			}
			// The Add(...) method adjusts both the monotonic and wall clocks
			v.timeZero = now.Add(-elapsedWallTime)
		}
	}
	return v.rotateEpochIfNeeded(false)
}

// Verify message.
// If payload is encrypted, returns the plaintext. Otherwise extracts and returns the payload as-is.
func (v *Verifier) Verify(message *universal.RoutableMessage) (plaintext []byte, err error) {
	v.lock.Lock()
	defer v.lock.Unlock()

	if err = v.adjustClock(); err != nil {
		return nil, err
	}

	if message.GetSignatureData() == nil {
		return nil, newError(errCodeBadParameter, "signature data missing")
	}
	var counter uint32

	switch sigData := message.GetSignatureData().SigType.(type) {
	case *signatures.SignatureData_AES_GCM_PersonalizedData:
		counter = sigData.AES_GCM_PersonalizedData.GetCounter()
		plaintext, err = v.verifyGCM(message, sigData.AES_GCM_PersonalizedData)
	case *signatures.SignatureData_HMAC_PersonalizedData:
		counter = sigData.HMAC_PersonalizedData.GetCounter()
		plaintext, err = v.verifyHMAC(message, sigData.HMAC_PersonalizedData)
	default:
		return nil, newError(errCodeBadParameter, "unrecognized authentication method")
	}

	if err != nil {
		return nil, err
	}

	if counter > 0 {
		var ok bool
		if v.counter, v.window, ok = updateSlidingWindow(v.counter, v.window, counter); !ok {
			return nil, v.signatureError(universal.MessageFault_E_MESSAGEFAULT_ERROR_INVALID_TOKEN_OR_COUNTER, message.GetUuid())
		}
	}

	return
}

// updateSlidingWindow takes the current counter value (i.e., the highest
// counter value of any authentic message received so far), the current sliding
// window, and the newCounter value from an incoming message. The function
// returns the updated counter and window values and sets ok to true if it
// could confirm that newCounter has never been previously used. If ok is
// false, then updatedCounter = counter and updatedWindow = window.
func updateSlidingWindow(counter uint32, window uint64, newCounter uint32) (updatedCounter uint32, updatedWindow uint64, ok bool) {
	// If we exit early due to an error, we want to leave the counter/window
	// state unchanged. Therefore we initialize return values to the current
	// state.
	updatedCounter = counter
	updatedWindow = window
	ok = false

	if counter == newCounter {
		// This counter value has been used before.
		return
	}

	if newCounter < counter {
		// This message arrived out of order.
		age := counter - newCounter
		if age > windowSize {
			// Our history doesn't go back this far, so we can't determine if
			// we've seen this newCounter value before.
			return
		}
		if window>>(age-1)&1 == 1 {
			// The newCounter value has been used before.
			return
		}
		// Everything looks good.
		ok = true
		updatedWindow |= (1 << (age - 1))
		return
	}

	// If we've reached this point, newCounter > counter, so newCounter is valid.
	ok = true
	updatedCounter = newCounter
	// Compute how far we need to shift our sliding window.
	shiftCount := newCounter - counter
	updatedWindow <<= shiftCount
	// We need to set the bit in our window that corresponds to counter (if
	// newCounter = counter + 1, then this is the first [LSB] of the window).
	updatedWindow |= uint64(1) << (shiftCount - 1)
	return
}

func (v *Verifier) verifyGCM(message *universal.RoutableMessage, gcmData *signatures.AES_GCM_Personalized_Signature_Data) (plaintext []byte, err error) {
	if err = v.verifySessionInfo(message, gcmData); err != nil {
		return nil, err
	}

	meta := newMetadata()
	if err := v.extractMetadata(meta, message, gcmData, signatures.SignatureType_SIGNATURE_TYPE_AES_GCM_PERSONALIZED); err != nil {
		return nil, err
	}

	plaintext, err = v.session.Decrypt(
		gcmData.GetNonce(), message.GetProtobufMessageAsBytes(), meta.Checksum(nil), gcmData.GetTag())

	if err != nil {
		return nil, v.signatureError(universal.MessageFault_E_MESSAGEFAULT_ERROR_INVALID_SIGNATURE, message.GetUuid())
	}

	return
}

func (v *Verifier) verifyHMAC(message *universal.RoutableMessage, hmacData *signatures.HMAC_Personalized_Signature_Data) (plaintext []byte, err error) {
	if err = v.verifySessionInfo(message, hmacData); err != nil {
		return nil, err
	}

	expectedTag, err := v.hmacTag(message, hmacData)
	if err != nil {
		return nil, err
	}

	if !hmac.Equal(hmacData.Tag, expectedTag) {
		return nil, v.signatureError(universal.MessageFault_E_MESSAGEFAULT_ERROR_INVALID_SIGNATURE, message.GetUuid())
	}

	return message.GetProtobufMessageAsBytes(), nil
}

func (v *Verifier) verifySessionInfo(message *universal.RoutableMessage, info sessionInfo) error {
	if domain := message.GetToDestination().GetDomain(); domain != v.domain && v.domain != universal.Domain_DOMAIN_BROADCAST {
		return newError(errCodeInvalidDomain, "wrong domain")
	}

	epoch := info.GetEpoch()
	if epoch != nil {
		if !bytes.Equal(epoch[:], v.epoch[:]) {
			return v.signatureError(universal.MessageFault_E_MESSAGEFAULT_ERROR_INCORRECT_EPOCH, message.GetUuid())
		}
	}

	expiresAt := info.GetExpiresAt()
	if expiresAt != 0 && expiresAt < v.timestamp() {
		return v.signatureError(universal.MessageFault_E_MESSAGEFAULT_ERROR_TIME_EXPIRED, message.GetUuid())
	}

	if time.Duration(expiresAt) > epochLength/time.Second {
		return v.signatureError(universal.MessageFault_E_MESSAGEFAULT_ERROR_BAD_PARAMETER, message.GetUuid())
	}

	// A counter value of zero disables the counter check, allow messages to
	// arrive out-of-order. This should only be used for messages that can
	// safely be replayed, that are likely to arrive out of order, and that
	// have a short expiration time.
	counter := info.GetCounter()
	if counter == 0 || counter < v.counter {
		// A message that arrives out of order must have a short expiration time remaining.
		if expiresAt == 0 || expiresAt-v.timestamp() > maxSecondsWithoutCounter {
			return v.signatureError(universal.MessageFault_E_MESSAGEFAULT_ERROR_TIME_TO_LIVE_TOO_LONG, message.GetUuid())
		}
	}
	return nil
}
