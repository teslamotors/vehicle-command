package vehicle

// This file implements commands targeting the Vehicle Security Controller
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

func unmarshalVCSECResponse(message *universal.RoutableMessage) (*vcsec.FromVCSECMessage, error) {
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
		return nil, protocol.NewError("payload missing from vehicle response", true, false)
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
			fromVCSEC, err := unmarshalVCSECResponse(reply)
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

const slotNone = 0xFFFFFFFF

func (v *Vehicle) getVCSECInfo(ctx context.Context, requestType vcsec.InformationRequestType, keySlot uint32) (*vcsec.FromVCSECMessage, error) {
	payload := vcsec.UnsignedMessage{
		SubMessage: &vcsec.UnsignedMessage_InformationRequest{
			InformationRequest: &vcsec.InformationRequest{
				InformationRequestType: requestType,
			},
		},
	}
	if keySlot != slotNone {
		payload.GetInformationRequest().Key = &vcsec.InformationRequest_Slot{
			Slot: keySlot,
		}
	}

	encodedPayload, err := proto.Marshal(&payload)
	if err != nil {
		return nil, err
	}

	done := func(v *vcsec.FromVCSECMessage) (bool, error) { return true, nil }
	return v.getVCSECResult(ctx, encodedPayload, connector.AuthMethodNone, done)
}

func isWhitelistOperationComplete(fromVCSEC *vcsec.FromVCSECMessage) (bool, error) {
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
	_, err := v.getVCSECResult(ctx, payload, v.authMethod, isWhitelistOperationComplete)
	return err
}

func addKeyPayload(publicKey *ecdh.PublicKey, role keys.Role, formFactor vcsec.KeyFormFactor) *vcsec.UnsignedMessage {
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

func (v *Vehicle) RemoteDrive(ctx context.Context) error {
	return v.executeRKEAction(ctx, vcsec.RKEAction_E_RKE_ACTION_REMOTE_DRIVE)
}

func (v *Vehicle) AutoSecureVehicle(ctx context.Context) error {
	return v.executeRKEAction(ctx, vcsec.RKEAction_E_RKE_ACTION_AUTO_SECURE_VEHICLE)
}

type Closure string

const (
	ClosureTrunk   Closure = "trunk"
	ClosureFrunk   Closure = "frunk"
	ClosureTonneau Closure = "tonneau"
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
	case ClosureTonneau:
		request.Tonneau = action
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
