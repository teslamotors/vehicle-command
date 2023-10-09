package vehicle

// This file implements commands targetting the Vehicle Security Controller
// domain (VCSEC). VCSEC handles key management and most vehicle actuators,
// such as the trunk and door locks.

import (
	"context"
	"crypto/ecdh"
	"fmt"
	"time"

	"github.com/teslamotors/vehicle-command/pkg/connector"
	"github.com/teslamotors/vehicle-command/pkg/protocol"

	"google.golang.org/protobuf/proto"

	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/keys"
	universal "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/universalmessage"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/vcsec"
)

func UnmarshalVCSECResponse(message *universal.RoutableMessage) (*vcsec.FromVCSECMessage, error) {
	// Handle protocol-level errors
	if err := protocol.GetError(message); err != nil {
		return nil, err
	}

	switch message.Payload.(type) {
	case *universal.RoutableMessage_ProtobufMessageAsBytes:
		// Continue
	case nil:
		return &vcsec.FromVCSECMessage{}, nil
	default:
		return nil, protocol.NewError("payload missing from vehicle respone", true, false)
	}
	encodedMessage := message.GetProtobufMessageAsBytes()

	// Handle application-layer errors
	var fromVCSEC vcsec.FromVCSECMessage
	if err := proto.Unmarshal(encodedMessage, &fromVCSEC); err != nil {
		return nil, &protocol.CommandError{Err: fmt.Errorf("%w: %s", protocol.ErrBadResponse, err), PossibleSuccess: true, PossibleTemporary: false}
	}
	if errMsg := fromVCSEC.GetNominalError(); errMsg != nil {
		return nil, &protocol.NominalError{Details: &protocol.NominalVCSECError{Details: errMsg}}
	}
	if status := fromVCSEC.GetCommandStatus(); status != nil {
		switch status.GetOperationStatus() {
		case vcsec.OperationStatus_E_OPERATIONSTATUS_OK:
		case vcsec.OperationStatus_E_OPERATIONSTATUS_WAIT:
			return nil, protocol.ErrBusy
		case vcsec.OperationStatus_E_OPERATIONSTATUS_ERROR:
			if code := status.GetWhitelistOperationStatus().GetWhitelistOperationInformation(); code != vcsec.WhitelistOperationInformation_E_WHITELISTOPERATION_INFORMATION_NONE {
				return nil, &protocol.KeychainError{Code: code}
			}
			if status.GetSignedMessageStatus() == nil {
				return nil, protocol.ErrUnknown
			}
		}
	}
	return &fromVCSEC, nil
}

type isTerminalTest func(fromVCSEC *vcsec.FromVCSECMessage) (bool, error)

// readUntil reads messages from VCSEC until one of them causes done to return true.
func readUntil(ctx context.Context, recv protocol.Receiver, done isTerminalTest) (*vcsec.FromVCSECMessage, error) {
	for {
		select {
		case reply := <-recv.Recv():
			fromVCSEC, err := UnmarshalVCSECResponse(reply)
			if err != nil {
				return nil, err
			}
			if ok, err := done(fromVCSEC); ok {
				return fromVCSEC, err
			}
		case <-ctx.Done():
			return nil, &protocol.CommandError{Err: ctx.Err(), PossibleSuccess: true, PossibleTemporary: true}
		}
	}
}

// getVCSECResult sends a payload to VCSEC, retrying as appropriate, and returns nil if the command succeeded.
func (v *Vehicle) getVCSECResult(ctx context.Context, payload []byte, auth connector.AuthMethod, done isTerminalTest) (*vcsec.FromVCSECMessage, error) {
	var fromVCSEC *vcsec.FromVCSECMessage
	for {
		recv, err := v.getReceiver(ctx, universal.Domain_DOMAIN_VEHICLE_SECURITY, payload, auth)
		if err == nil {
			fromVCSEC, err = readUntil(ctx, recv, done)
			recv.Close()
		}

		if !protocol.ShouldRetry(err) {
			return fromVCSEC, err
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(v.dispatcher.RetryInterval()):
			continue
		}
	}
}

func IsWhitelistOperationComplete(fromVCSEC *vcsec.FromVCSECMessage) (bool, error) {
	if opStatus := fromVCSEC.GetCommandStatus().GetWhitelistOperationStatus(); opStatus != nil {
		status := opStatus.GetWhitelistOperationInformation()
		if status == vcsec.WhitelistOperationInformation_E_WHITELISTOPERATION_INFORMATION_NONE {
			return true, nil
		}
		// This code should be unreachable if VCSEC sends correctly formed
		// messages (i.e., if the operation status is set to error whenever
		// code indicates a fault).
		return true, &protocol.KeychainError{Code: status}
	}
	return false, nil
}

func (v *Vehicle) executeWhitelistOperation(ctx context.Context, payload []byte) error {
	_, err := v.getVCSECResult(ctx, payload, v.authMethod, IsWhitelistOperationComplete)
	return err
}

func (v *Vehicle) RemoveKey(ctx context.Context, publicKey *ecdh.PublicKey) error {
	if publicKey.Curve() != ecdh.P256() {
		return protocol.ErrInvalidPublicKey
	}
	payload := vcsec.UnsignedMessage{
		SubMessage: &vcsec.UnsignedMessage_WhitelistOperation{
			WhitelistOperation: &vcsec.WhitelistOperation{
				SubMessage: &vcsec.WhitelistOperation_RemovePublicKeyFromWhitelist{
					RemovePublicKeyFromWhitelist: &vcsec.PublicKey{
						PublicKeyRaw: publicKey.Bytes(),
					},
				},
			},
		},
	}
	encodedPayload, err := proto.Marshal(&payload)
	if err != nil {
		return err
	}
	return v.executeWhitelistOperation(ctx, encodedPayload)
}

func (v *Vehicle) KeySummary(ctx context.Context) (*vcsec.WhitelistInfo, error) {
	payload := vcsec.UnsignedMessage{
		SubMessage: &vcsec.UnsignedMessage_InformationRequest{
			InformationRequest: &vcsec.InformationRequest{
				InformationRequestType: vcsec.InformationRequestType_INFORMATION_REQUEST_TYPE_GET_WHITELIST_INFO,
			},
		},
	}
	encodedPayload, err := proto.Marshal(&payload)
	if err != nil {
		return nil, err
	}
	done := func(v *vcsec.FromVCSECMessage) (bool, error) { return true, nil }
	reply, err := v.getVCSECResult(ctx, encodedPayload, connector.AuthMethodNone, done)
	if err != nil {
		return nil, err
	}
	return reply.GetWhitelistInfo(), err
}

func (v *Vehicle) KeyInfoBySlot(ctx context.Context, slot uint32) (*vcsec.WhitelistEntryInfo, error) {
	payload := vcsec.UnsignedMessage{
		SubMessage: &vcsec.UnsignedMessage_InformationRequest{
			InformationRequest: &vcsec.InformationRequest{
				InformationRequestType: vcsec.InformationRequestType_INFORMATION_REQUEST_TYPE_GET_WHITELIST_ENTRY_INFO,
				Key: &vcsec.InformationRequest_Slot{
					Slot: slot,
				},
			},
		},
	}
	encodedPayload, err := proto.Marshal(&payload)
	if err != nil {
		return nil, err
	}
	done := func(v *vcsec.FromVCSECMessage) (bool, error) { return true, nil }
	reply, err := v.getVCSECResult(ctx, encodedPayload, connector.AuthMethodNone, done)
	if err != nil {
		return nil, err
	}
	return reply.GetWhitelistEntryInfo(), err
}

func addKeyPayload(publicKey *ecdh.PublicKey, isOwner bool, formFactor vcsec.KeyFormFactor) *vcsec.UnsignedMessage {
	var role keys.Role
	if isOwner {
		role = keys.Role_ROLE_OWNER
	} else {
		role = keys.Role_ROLE_DRIVER
	}
	return &vcsec.UnsignedMessage{
		SubMessage: &vcsec.UnsignedMessage_WhitelistOperation{
			WhitelistOperation: &vcsec.WhitelistOperation{
				SubMessage: &vcsec.WhitelistOperation_AddKeyToWhitelistAndAddPermissions{
					AddKeyToWhitelistAndAddPermissions: &vcsec.PermissionChange{
						Key: &vcsec.PublicKey{
							PublicKeyRaw: publicKey.Bytes(),
						},
						KeyRole: role,
					},
				},
				MetadataForKey: &vcsec.KeyMetadata{
					KeyFormFactor: formFactor,
				},
			},
		},
	}
}

// SendAddKeyRequest sends an add-key request to the vehicle over BLE. The user must approve the
// request by tapping their NFC card on the center console and then confirming their intent on the
// vehicle UI.
//
// If isOwner is true, the new key can authorize changes to vehicle access controls, such as
// adding/removing other keys.
//
// This function returns nil as soon as the request is transmitted. A nil return value does not
// guarantee the user has approved the request.
//
// Clients can check if publicKey has been enrolled and synchronized with the infotainment system by
// attempting to call v.SessionInfo with the domain argument set to
// [universal.Domain_DOMAIN_INFOTAINMENT].
func (v *Vehicle) SendAddKeyRequest(ctx context.Context, publicKey *ecdh.PublicKey, isOwner bool, formFactor vcsec.KeyFormFactor) error {
	if publicKey.Curve() != ecdh.P256() {
		return protocol.ErrInvalidPublicKey
	}
	if _, ok := v.conn.(connector.FleetAPIConnector); ok {
		return protocol.ErrRequiresBLE
	}
	encodedPayload, err := proto.Marshal(addKeyPayload(publicKey, isOwner, formFactor))
	if err != nil {
		return err
	}
	envelope := &vcsec.ToVCSECMessage{
		SignedMessage: &vcsec.SignedMessage{
			ProtobufMessageAsBytes: encodedPayload,
			SignatureType:          vcsec.SignatureType_SIGNATURE_TYPE_PRESENT_KEY,
		},
	}
	encodedEnvelope, err := proto.Marshal(envelope)
	if err != nil {
		return err
	}
	return v.conn.Send(ctx, encodedEnvelope)
}

// AddKey adds a public key to the vehicle's whitelist. If isOwner is true, the new key can
// authorize changes to vehicle access controls, such as adding/removing other keys.
func (v *Vehicle) AddKey(ctx context.Context, publicKey *ecdh.PublicKey, isOwner bool, formFactor vcsec.KeyFormFactor) error {
	if publicKey.Curve() != ecdh.P256() {
		return protocol.ErrInvalidPublicKey
	}
	payload := addKeyPayload(publicKey, isOwner, formFactor)
	encodedPayload, err := proto.Marshal(payload)
	if err != nil {
		return err
	}
	return v.executeWhitelistOperation(ctx, encodedPayload)
}

// executeRKEAction sends an RKE action command to the vehicle. (RKE originally
// referred to "Remote Keyless Entry" but now refers more generally to commands
// that can be sent by a keyfob).
func (v *Vehicle) executeRKEAction(ctx context.Context, action vcsec.RKEAction_E) error {
	done := func(fromVCSEC *vcsec.FromVCSECMessage) (bool, error) {
		if fromVCSEC.GetCommandStatus() == nil {
			return true, nil
		}
		return false, nil
	}

	payload := vcsec.UnsignedMessage{
		SubMessage: &vcsec.UnsignedMessage_RKEAction{
			RKEAction: action,
		},
	}
	encodedPayload, err := proto.Marshal(&payload)
	if err != nil {
		return err
	}

	_, err = v.getVCSECResult(ctx, encodedPayload, v.authMethod, done)
	return err
}

// Not exported. Use v.Wakeup instead, which chooses the correct wake method based on available transport.
func (v *Vehicle) wakeupRKE(ctx context.Context) error {
	return v.executeRKEAction(ctx, vcsec.RKEAction_E_RKE_ACTION_WAKE_VEHICLE)
}

func (v *Vehicle) Lock(ctx context.Context) error {
	return v.executeRKEAction(ctx, vcsec.RKEAction_E_RKE_ACTION_LOCK)
}

func (v *Vehicle) Unlock(ctx context.Context) error {
	return v.executeRKEAction(ctx, vcsec.RKEAction_E_RKE_ACTION_UNLOCK)
}

func (v *Vehicle) RemoteDrive(ctx context.Context) error {
	return v.executeRKEAction(ctx, vcsec.RKEAction_E_RKE_ACTION_REMOTE_DRIVE)
}

func (v *Vehicle) AutoSecureVehicle(ctx context.Context) error {
	return v.executeRKEAction(ctx, vcsec.RKEAction_E_RKE_ACTION_AUTO_SECURE_VEHICLE)
}

type Closure string

const (
	ClosureTrunk Closure = "trunk"
	ClosureFrunk Closure = "frunk"
)

func (v *Vehicle) executeClosureAction(ctx context.Context, action vcsec.ClosureMoveType_E, closure Closure) error {
	done := func(fromVCSEC *vcsec.FromVCSECMessage) (bool, error) {
		if fromVCSEC.GetCommandStatus() == nil {
			return true, nil
		}
		return false, nil
	}

	// Not all actions are meaningful for all closures. Exported methods restrict combinations.
	var request vcsec.ClosureMoveRequest
	switch closure {
	case ClosureTrunk:
		request.RearTrunk = action
	case ClosureFrunk:
		request.FrontTrunk = action
	}

	payload := vcsec.UnsignedMessage{
		SubMessage: &vcsec.UnsignedMessage_ClosureMoveRequest{
			ClosureMoveRequest: &request,
		},
	}

	encodedPayload, err := proto.Marshal(&payload)
	if err != nil {
		return err
	}

	_, err = v.getVCSECResult(ctx, encodedPayload, v.authMethod, done)
	return err
}

func (v *Vehicle) ActuateTrunk(ctx context.Context) error {
	return v.executeClosureAction(ctx, vcsec.ClosureMoveType_E_CLOSURE_MOVE_TYPE_MOVE, ClosureTrunk)
}

// OpenTrunk opens the trunk, but note that CloseTrunk is not available on all vehicle types.
func (v *Vehicle) OpenTrunk(ctx context.Context) error {
	return v.executeClosureAction(ctx, vcsec.ClosureMoveType_E_CLOSURE_MOVE_TYPE_OPEN, ClosureTrunk)
}

// CloseTrunk is not available on all vehicle types.
func (v *Vehicle) CloseTrunk(ctx context.Context) error {
	return v.executeClosureAction(ctx, vcsec.ClosureMoveType_E_CLOSURE_MOVE_TYPE_CLOSE, ClosureTrunk)
}

// OpenTrunk opens the frunk. There is no remote way to close the frunk!
func (v *Vehicle) OpenFrunk(ctx context.Context) error {
	return v.executeClosureAction(ctx, vcsec.ClosureMoveType_E_CLOSURE_MOVE_TYPE_OPEN, ClosureFrunk)
}
