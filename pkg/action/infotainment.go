package action

import (
	"fmt"
	"time"

	carserver "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/carserver"
)

// Ping sends an authenticated "no-op" command to the vehicle.
// If the method returns an non-nil error, then the vehicle is online and recognizes the client's
// public key.
//
// The error is a [protocol.RoutableMessageError] then the vehicle is online, but rejected the command
// for some other reason (for example, it may not recognize the client's public key or may have
// mobile access disabled).
func Ping() *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_Ping{
				Ping: &carserver.Ping{
					PingId: 1, // Responses are disambiguated on the protocol later
				},
			},
		},
	}
}

// SetVolume to a value between 0 and 10.
func SetVolume(volume float32) (*carserver.Action_VehicleAction, error) {
	if volume < 0 || volume > 10 {
		return nil, fmt.Errorf("invalid volume (should be in [0, 10])")
	}
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_MediaUpdateVolume{
				MediaUpdateVolume: &carserver.MediaUpdateVolume{
					MediaVolume: &carserver.MediaUpdateVolume_VolumeAbsoluteFloat{
						VolumeAbsoluteFloat: volume,
					},
				},
			},
		},
	}, nil
}

// ToggleMediaPlayback toggles media pause/play state.
func ToggleMediaPlayback() *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_MediaPlayAction{
				MediaPlayAction: &carserver.MediaPlayAction{},
			},
		},
	}
}

// ScheduleSoftwareUpdate schedules a software update to start after a delay.
func ScheduleSoftwareUpdate(delay time.Duration) *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_VehicleControlScheduleSoftwareUpdateAction{
				VehicleControlScheduleSoftwareUpdateAction: &carserver.VehicleControlScheduleSoftwareUpdateAction{
					OffsetSec: int32(delay.Seconds()),
				},
			},
		},
	}
}

// CancelSoftwareUpdate cancels a pending software update.
func CancelSoftwareUpdate() *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_VehicleControlCancelSoftwareUpdateAction{
				VehicleControlCancelSoftwareUpdateAction: &carserver.VehicleControlCancelSoftwareUpdateAction{},
			},
		},
	}
}

// SetVehicleName sets the vehicle name.
func SetVehicleName(name string) *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_SetVehicleNameAction{
				SetVehicleNameAction: &carserver.SetVehicleNameAction{
					VehicleName: name,
				},
			},
		},
	}
}
