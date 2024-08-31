package vehicle

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/proto"

	"github.com/teslamotors/vehicle-command/pkg/protocol"
	carserver "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/carserver"
	universal "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/universalmessage"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/vcsec"
)

// ExecuteAction executes an action on the vehicle.
//
// The action can be a *carserver.Action_VehicleAction or a *vcsec.UnsignedMessage.
// Actions are created using the action package.
func (v *Vehicle) ExecuteAction(ctx context.Context, action interface{}) error {
	switch action := action.(type) {
	case *carserver.Action_VehicleAction:
		_, err := v.getCarServerResponse(ctx, action)
		return err
	case *vcsec.UnsignedMessage:
		encodedPayload, err := proto.Marshal(action)
		if err != nil {
			return err
		}

		_, err = v.getVCSECResult(ctx, encodedPayload, v.authMethod, isUnsignedActionDone)
		return err
	default:
		return fmt.Errorf("unsupported action type: %T", action)
	}
}

func isUnsignedActionDone(fromVCSEC *vcsec.FromVCSECMessage) (bool, error) {
	if fromVCSEC.GetCommandStatus() == nil {
		return true, nil
	}
	return false, nil
}

func (v *Vehicle) getCarServerResponse(ctx context.Context, action *carserver.Action_VehicleAction) (*carserver.Response, error) {
	payload := carserver.Action{
		ActionMsg: action,
	}
	encodedPayload, err := proto.Marshal(&payload)
	if err != nil {
		return nil, err
	}
	responsePayload, err := v.Send(ctx, universal.Domain_DOMAIN_INFOTAINMENT, encodedPayload, v.authMethod)
	if err != nil {
		return nil, err
	}

	var response carserver.Response
	if err := proto.Unmarshal(responsePayload, &response); err != nil {
		return nil, &protocol.CommandError{Err: fmt.Errorf("unable to parse vehicle response: %w", err), PossibleSuccess: true, PossibleTemporary: false}
	}

	if response.GetActionStatus().GetResult() == carserver.OperationStatus_E_OPERATIONSTATUS_ERROR {
		description := response.GetActionStatus().GetResultReason().GetPlainText()
		if description == "" {
			description = "unspecified error"
		}
		return nil, &protocol.NominalError{Details: protocol.NewError("car could not execute command: "+description, false, false)}
	}
	return &response, nil
}
