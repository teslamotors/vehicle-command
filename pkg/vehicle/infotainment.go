// File implements helper functions for commands that terminate on infotainment.
//
// File also contains misc. commands that don't fall cleanly into more specific categories.

package vehicle

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/teslamotors/vehicle-command/pkg/protocol"
	carserver "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/carserver"
	universal "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/universalmessage"
)

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

func (v *Vehicle) executeCarServerAction(ctx context.Context, action *carserver.Action_VehicleAction) error {
	_, err := v.getCarServerResponse(ctx, action)
	return err
}

// Ping sends an authenticated "no-op" command to the vehicle.
// If the method returns an non-nil error, then the vehicle is online and recognizes the client's
// public key.
//
// The error is a [protocol.RoutableMessageError] then the vehicle is online, but rejected the command
// for some other reason (for example, it may not recognize the client's public key or may have
// mobile access disabled).
func (v *Vehicle) Ping(ctx context.Context) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_Ping{
					Ping: &carserver.Ping{
						PingId: 1, // Responses are disambiguated on the protocol later
					},
				},
			},
		})
}

// SetVolume to a value between 0 and 10.
func (v *Vehicle) SetVolume(ctx context.Context, volume float32) error {
	if volume < 0 || volume > 10 {
		return fmt.Errorf("invalid volume (should be in [0, 10])")
	}
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_MediaUpdateVolume{
					MediaUpdateVolume: &carserver.MediaUpdateVolume{
						MediaVolume: &carserver.MediaUpdateVolume_VolumeAbsoluteFloat{
							VolumeAbsoluteFloat: volume,
						},
					},
				},
			},
		})
}

func (v *Vehicle) ToggleMediaPlayback(ctx context.Context) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_MediaPlayAction{
					MediaPlayAction: &carserver.MediaPlayAction{},
				},
			},
		})
}

func (v *Vehicle) ScheduleSoftwareUpdate(ctx context.Context, delay time.Duration) error {
	seconds := int32(delay / time.Second)
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_VehicleControlScheduleSoftwareUpdateAction{
					VehicleControlScheduleSoftwareUpdateAction: &carserver.VehicleControlScheduleSoftwareUpdateAction{
						OffsetSec: seconds,
					},
				},
			},
		})
}

func (v *Vehicle) CancelSoftwareUpdate(ctx context.Context) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_VehicleControlCancelSoftwareUpdateAction{
					VehicleControlCancelSoftwareUpdateAction: &carserver.VehicleControlCancelSoftwareUpdateAction{},
				},
			},
		})
}

type SeatPosition int64

// Enumerated type for seats. Values with the Back suffix are used for seat heater/cooler commands,
// and refer to the backrest. Backrest heaters are only available on some Model S vehicles.
const (
	SeatUnknown SeatPosition = iota
	SeatFrontLeft
	SeatFrontRight
	SeatSecondRowLeft
	SeatSecondRowLeftBack
	SeatSecondRowCenter
	SeatSecondRowRight
	SeatSecondRowRightBack
	SeatThirdRowLeft
	SeatThirdRowRight
)

func (v *Vehicle) GetNearbyCharging(ctx context.Context) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_GetNearbyChargingSites{
					GetNearbyChargingSites: &carserver.GetNearbyChargingSites{
						IncludeMetaData: true,
						Radius:          200,
						Count:           10,
					},
				},
			},
		})
}

func (v *Vehicle) SetVehicleName(ctx context.Context, name string) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_SetVehicleNameAction{
					SetVehicleNameAction: &carserver.SetVehicleNameAction{
						VehicleName: name,
					},
				},
			},
		})
}
