package authentication

import (
	"bytes"
	"crypto/hmac"
	"time"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"

	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/signatures"
	universal "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/universalmessage"
)

// Signers encrypt messages that are decrypted and verified by a designated Verifier.
// (Technically speaking, the name is a misnomer since Signers use symmetric-key operations
// following ECDH key agreement.)
type Signer struct {
	Peer
	verifierPublicBytes []byte
	setTime             uint32 // Transmission time (according to Verifier clock) of the last transmitted session info known to the Signer.
	SupportsDetached    bool   // Whether the Verifier supports detached signatures.
}

// NewSigner creates a Signer that sends authenticated messages to the Verifier named verifierName.
// In order to use this function, the client needs to obtain verifierInfo from the Verifier.
func NewSigner(private ECDHPrivateKey, verifierName []byte, verifierInfo *signatures.SessionInfo) (*Signer, error) {
	if len(verifierName) > 255 {
		return nil, ErrMetadataFieldTooLong
	}
	session, err := private.Exchange(verifierInfo.GetPublicKey())
	if err != nil {
		return nil, err
	}
	signer := Signer{
		Peer: Peer{
			verifierName: verifierName,
			session:      session,
			counter:      verifierInfo.GetCounter(),
			timeZero:     epochStartTime(verifierInfo.GetClockTime()),
		},
		setTime:             verifierInfo.GetClockTime(),
		verifierPublicBytes: verifierInfo.GetPublicKey(),
	}
	copy(signer.epoch[:], verifierInfo.GetEpoch())

	return &signer, nil
}

// NewAuthenticatedSigner creates a Signer from encoded and cryptographically verified session info.
func NewAuthenticatedSigner(private ECDHPrivateKey, verifierName, challenge, encodedInfo, tag []byte) (*Signer, error) {
	signer, err := ImportSessionInfo(private, verifierName, encodedInfo, time.Now())
	if err != nil {
		return nil, err
	}
	validTag, err := signer.session.SessionInfoHMAC(verifierName, challenge, encodedInfo)
	if err != nil {
		return nil, err
	}
	if !hmac.Equal(validTag, tag) {
		return nil, newError(errCodeInvalidSignature, "session info hmac invalid")
	}
	return signer, nil
}

// RemotePublicKeyBytes returns the Verifer's public key encoded without point compression.
func (s *Signer) RemotePublicKeyBytes() []byte {
	return append([]byte{}, s.verifierPublicBytes...)
}

// ImportSessionInfo allows creation of a Signer with cached SessionInfo.
// This can be used to avoid a round trip with the Verifier.
func ImportSessionInfo(private ECDHPrivateKey, verifierName, encodedInfo []byte, generatedAt time.Time) (*Signer, error) {
	var info signatures.SessionInfo
	if err := proto.Unmarshal(encodedInfo, &info); err != nil {
		return nil, newError(errCodeDecoding, "invalid session info protobuf")
	}
	signer, err := NewSigner(private, verifierName, &info)
	if err != nil {
		return nil, err
	}
	signer.timeZero = generatedAt.Add(-time.Duration(info.ClockTime) * time.Second)
	signer.SupportsDetached = info.GetFlags()&(1<<signatures.SessionInfoFlags_SESSION_INFO_FLAG_SUPPORTS_DETACHED_ENCRYPTION) != 0
	return signer, nil
}

// ExportSessionInfo can be used to write session state to disk, allowing for later resumption using
// ImportSessionInfo.
func (s *Signer) ExportSessionInfo() ([]byte, error) {
	info := signatures.SessionInfo{
		Counter:   s.counter,
		PublicKey: s.verifierPublicBytes[:],
		Epoch:     s.epoch[:],
		ClockTime: s.timestamp(),
	}
	if s.SupportsDetached {
		info.Flags |= 1 << signatures.SessionInfoFlags_SESSION_INFO_FLAG_SUPPORTS_DETACHED_ENCRYPTION
	}
	return proto.Marshal(&info)
}

// UpdateSessionInfo allows s to resync session state with a Verifier.
// A Verifier may include info in an authentication error message when the error may have resulted
// from a desync. The Signer can update its session info and then reattempt transmission.
func (s *Signer) UpdateSessionInfo(info *signatures.SessionInfo) error {
	if !bytes.Equal(info.GetPublicKey(), s.verifierPublicBytes) {
		return newError(errCodeUnknownKey, "public key in SessionInfo doesn't match value used to initialize Signer")
	}
	if !bytes.Equal(s.epoch[:], info.Epoch) || (s.setTime <= info.ClockTime) {
		if s.counter < info.Counter {
			s.counter = info.Counter
		}
		copy(s.epoch[:], info.Epoch)
		s.setTime = info.ClockTime
		s.timeZero = epochStartTime(info.ClockTime)
	}
	return nil
}

// UpdateSignedSessionInfo allows s to resync session state with a Verifier using cryptographically
// verified session state.
// See UpdateSessionInfo.
func (s *Signer) UpdateSignedSessionInfo(challenge, encodedInfo, tag []byte) error {
	validTag, err := s.session.SessionInfoHMAC(s.verifierName, challenge, encodedInfo)
	if err != nil {
		return err
	}
	if !hmac.Equal(validTag, tag) {
		return newError(errCodeInvalidSignature, "session info hmac invalid")
	}
	var info signatures.SessionInfo
	if err := proto.Unmarshal(encodedInfo, &info); err != nil {
		return newError(errCodeDecoding, "invalid session info protobuf")
	}
	return s.UpdateSessionInfo(&info)
}

func (s *Signer) encryptWithCounter(message *universal.RoutableMessage, expiresIn time.Duration, counter uint32) error {
	var gcmData signatures.AES_GCM_Personalized_Signature_Data
	message.SubSigData = &universal.RoutableMessage_SignatureData{
		SignatureData: &signatures.SignatureData{
			SignerIdentity: &signatures.KeyIdentity{
				IdentityType: &signatures.KeyIdentity_PublicKey{
					PublicKey: s.session.LocalPublicBytes(),
				},
			},
			SigType: &signatures.SignatureData_AES_GCM_PersonalizedData{
				AES_GCM_PersonalizedData: &gcmData,
			},
		},
	}

	gcmData.Epoch = append(gcmData.Epoch, s.epoch[:]...)
	gcmData.Counter = counter
	gcmData.ExpiresAt = uint32(time.Now().Add(expiresIn).Sub(s.timeZero) / time.Second)

	meta := newMetadata()
	err := s.extractMetadata(meta, message, &gcmData, signatures.SignatureType_SIGNATURE_TYPE_AES_GCM_PERSONALIZED)
	if err != nil {
		return err
	}
	if plaintext, ok := message.Payload.(*universal.RoutableMessage_ProtobufMessageAsBytes); ok {
		var ciphertext []byte
		gcmData.Nonce, ciphertext, gcmData.Tag, err = s.session.Encrypt(
			plaintext.ProtobufMessageAsBytes, meta.Checksum(nil))
		message.Payload = &universal.RoutableMessage_ProtobufMessageAsBytes{ProtobufMessageAsBytes: ciphertext}
	} else {
		return newError(errCodeBadParameter, "Missing protobuf message")
	}
	return err
}

// Encrypt message's payload in-place.
// This method adds (authenticated) metadata to the message as well, including the provided
// expiration time.
func (s *Signer) Encrypt(message *universal.RoutableMessage, expiresIn time.Duration) error {
	if s.counter == 0xFFFFFFFF {
		return newError(errCodeInvalidToken, "counter rollover")
	}
	s.counter++
	return s.encryptWithCounter(message, expiresIn, s.counter)
}

// EncryptDetach keeps the high-level command as authenticated plaintext (e.g., "set pin") but
// encrypts the payload (e.g., "1234").
//
// BLE communications should instead use [Signer.Encrypt].
func (s *Signer) EncryptDetach(message *universal.RoutableMessage, expiresIn time.Duration) error {
	payload := message.GetProtobufMessageAsBytes()
	if payload == nil {
		return newError(errCodeBadParameter, "missing protobuf message")
	}
	depth := 0
	switch message.GetToDestination().GetDomain() {
	case universal.Domain_DOMAIN_INFOTAINMENT:
		depth = 2 // Infotainment: carserver.Action -> <service> -> <command>
	case universal.Domain_DOMAIN_VEHICLE_SECURITY:
		depth = 1 // VCSEC: vcsec.UnsignedMessage -> <command>
	default:
		return newError(errCodeBadParameter, "unsupported domain for EncryptDetach")
	}
	var fieldNumbers []protowire.Number
	for i := 0; i < depth; i++ {
		fieldNumber, wireType, n := protowire.ConsumeTag(payload)
		if n < 0 {
			return newError(errCodeDecoding, "invalid protobuf")
		}
		if wireType != protowire.BytesType {
			return newError(errCodeDecoding, "invalid protobuf command type")
		}
		fieldNumbers = append(fieldNumbers, fieldNumber)
		subCommand, subCommandLength := protowire.ConsumeBytes(payload[n:])
		if subCommandLength < 0 {
			return newError(errCodeDecoding, "invalid subcommand")
		}
		if subCommandLength+n != len(payload) {
			return newError(errCodeDecoding, "command includes data outside of innermost envelope")
		}
		payload = subCommand
	}

	// Separate out the cryptography from the semantic checks above so that unit tests can easily do
	// the former on semantically invalid payloads.
	return s.addDetachedAuthentication(message, marshalNestedMessage(fieldNumbers, nil), payload, expiresIn)
}

func (s *Signer) addDetachedAuthentication(message *universal.RoutableMessage, prefix, payload []byte, expiresIn time.Duration) error {
	s.counter++
	gcmData := signatures.AES_GCM_Detached_Signature_Data{
		Counter:   s.counter,
		ExpiresAt: uint32(time.Now().Add(expiresIn).Sub(s.timeZero) / time.Second),
		Prefix:    append([]byte{}, prefix...),
	}

	meta := newMetadata()
	if err := s.extractMetadata(meta, message, &gcmData, signatures.SignatureType_SIGNATURE_TYPE_AES_GCM_DETACHED); err != nil {
		return err
	}
	if err := meta.Add(signatures.Tag_TAG_COMMAND_PREFIX, gcmData.Prefix); err != nil {
		return err
	}

	var ciphertext []byte
	var err error
	gcmData.Nonce, ciphertext, gcmData.Tag, err = s.session.Encrypt(payload, meta.Checksum(nil))
	if err != nil {
		return err
	}
	message.Payload = &universal.RoutableMessage_ProtobufMessageAsBytes{
		ProtobufMessageAsBytes: ciphertext,
	}
	message.SubSigData = &universal.RoutableMessage_SignatureData{
		SignatureData: &signatures.SignatureData{
			SignerIdentity: &signatures.KeyIdentity{
				IdentityType: &signatures.KeyIdentity_PublicKey{
					PublicKey: s.session.LocalPublicBytes(),
				},
			},
			SigType: &signatures.SignatureData_AES_GCM_DetachedData{
				AES_GCM_DetachedData: &gcmData,
			},
		},
	}
	return nil
}

// AuthorizeHMAC adds an authentication tag to message.
//
// This allows the recipient to verify the message has not been tampered with. The HTTP proxy needs
// to inspect commands in order to enforce OAuth scopes and determine when a sequence of replies
// terminates, so the high-level command must remain plaintext. If the [Verifier] supports it, the
// Signer automatically upgrades to [Signer.EncryptDetach], which encrypts command parameters while
// leaving the command type in plaintext. If a client is not using the HTTP proxy, it should use
// Encrypt instead of AuthorizeHMAC.
//
// Sensitive data, such as live camera streams, is encrypted on the application layer.
func (s *Signer) AuthorizeHMAC(message *universal.RoutableMessage, expiresIn time.Duration) error {
	if s.SupportsDetached {
		return s.EncryptDetach(message, expiresIn)
	}
	s.counter++
	hmacData := signatures.HMAC_Personalized_Signature_Data{
		Counter:   s.counter,
		ExpiresAt: uint32(time.Now().Add(expiresIn).Sub(s.timeZero) / time.Second),
	}
	hmacData.Epoch = append(hmacData.Epoch, s.epoch[:]...)
	var err error
	hmacData.Tag, err = s.hmacTag(message, &hmacData)

	message.SubSigData = &universal.RoutableMessage_SignatureData{
		SignatureData: &signatures.SignatureData{
			SignerIdentity: &signatures.KeyIdentity{
				IdentityType: &signatures.KeyIdentity_PublicKey{
					PublicKey: s.session.LocalPublicBytes(),
				},
			},
			SigType: &signatures.SignatureData_HMAC_PersonalizedData{
				HMAC_PersonalizedData: &hmacData,
			},
		},
	}
	if err != nil {
		return err
	}
	return nil
}

// Decrypt a Verifier message in place.
//
// Returns the anti-replay counter included in the message, which the client must verify increases
// monotonically for a given id or is inside of a sliding window.
func (s *Signer) Decrypt(message *universal.RoutableMessage, id []byte) (uint32, error) {
	gcmInfo := message.GetSignatureData().GetAES_GCM_ResponseData()
	if gcmInfo == nil {
		return 0, newError(errCodeBadParameter, "missing AES-GCM data")
	}
	if len(gcmInfo.GetNonce()) != nonceLength {
		return 0, newError(universal.MessageFault_E_MESSAGEFAULT_ERROR_IV_INCORRECT_LENGTH, "invalid AES-GCM nonce length")
	}
	authenticatedData, err := s.responseMetadata(message, id, gcmInfo.Counter)
	if err != nil {
		return 0, err
	}
	plaintext, err := s.session.Decrypt(
		gcmInfo.Nonce,
		message.GetProtobufMessageAsBytes(),
		authenticatedData,
		gcmInfo.Tag,
	)
	if err != nil {
		return 0, err
	}
	message.Payload = &universal.RoutableMessage_ProtobufMessageAsBytes{
		ProtobufMessageAsBytes: plaintext,
	}
	message.SubSigData = nil
	return gcmInfo.Counter, nil
}
