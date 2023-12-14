// File implements commands that trigger physical vehicle actions.

package vehicle

import (
	"context"

	carserver "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/carserver"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/vcsec"
)

// OpenTrunk opens the trunk. This method requires either a powered trunk or firmware version
// 2024.14+. The command silently fails on other vehicles, but ActuateTrunk may be used instead.
//
// Note the CloseTrunk method requires a powered trunk. Check for "can_actuate_trunks" under
// "vehicle_config" in the response from the [Vehicle Data Fleet API endpoint] to determine the
// vehicle's capabilities.
//
// [Vehicle Data Fleet API endpoint]: https://developer.tesla.com/docs/tesla-fleet-api#vehicle_data
func (v *Vehicle) OpenTrunk(ctx context.Context) error {
	return v.executeClosureAction(ctx, vcsec.ClosureMoveType_E_CLOSURE_MOVE_TYPE_OPEN, ClosureTrunk)
}

// ActuateTrunk toggles the trunk between open and closed. Only vehicles with a powered trunk will
// close.
func (v *Vehicle) ActuateTrunk(ctx context.Context) error {
	return v.executeClosureAction(ctx, vcsec.ClosureMoveType_E_CLOSURE_MOVE_TYPE_MOVE, ClosureTrunk)
}

// CloseTrunk closes the trunk.
//
// This method requires a powered trunk. Check for "can_actuate_trunks" under "vehicle_config" in
// the response from the [Vehicle Data Fleet API endpoint] to determine the vehicle's capabilities.
func (v *Vehicle) CloseTrunk(ctx context.Context) error {
	return v.executeClosureAction(ctx, vcsec.ClosureMoveType_E_CLOSURE_MOVE_TYPE_CLOSE, ClosureTrunk)
}

// OpenFrunk opens the frunk. There is no remote way to close the frunk!
func (v *Vehicle) OpenFrunk(ctx context.Context) error {
	return v.executeRKEAction(ctx, vcsec.RKEAction_E_RKE_ACTION_OPEN_FRUNK)
}

func (v *Vehicle) HonkHorn(ctx context.Context) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_VehicleControlHonkHornAction{
					VehicleControlHonkHornAction: &carserver.VehicleControlHonkHornAction{},
				},
			},
		})
}

func (v *Vehicle) FlashLights(ctx context.Context) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_VehicleControlFlashLightsAction{
					VehicleControlFlashLightsAction: &carserver.VehicleControlFlashLightsAction{},
				},
			},
		})
}

func (v *Vehicle) ChangeSunroofState(ctx context.Context, sunroofLevel int32) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_VehicleControlSunroofOpenCloseAction{
					VehicleControlSunroofOpenCloseAction: &carserver.VehicleControlSunroofOpenCloseAction{
						SunroofLevel: &carserver.VehicleControlSunroofOpenCloseAction_AbsoluteLevel{
							AbsoluteLevel: sunroofLevel,
						},
					},
				},
			},
		})
}

func (v *Vehicle) CloseWindows(ctx context.Context) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_VehicleControlWindowAction{
					VehicleControlWindowAction: &carserver.VehicleControlWindowAction{
						Action: &carserver.VehicleControlWindowAction_Close{
							Close: &carserver.Void{},
						},
					},
				},
			},
		})
}

func (v *Vehicle) VentWindows(ctx context.Context) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_VehicleControlWindowAction{
					VehicleControlWindowAction: &carserver.VehicleControlWindowAction{
						Action: &carserver.VehicleControlWindowAction_Vent{
							Vent: &carserver.Void{},
						},
					},
				},
			},
		})
}

func (v *Vehicle) ChargePortClose(ctx context.Context) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_ChargePortDoorClose{
					ChargePortDoorClose: &carserver.ChargePortDoorClose{},
				},
			},
		})
}

func (v *Vehicle) ChargePortOpen(ctx context.Context) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_ChargePortDoorOpen{
					ChargePortDoorOpen: &carserver.ChargePortDoorOpen{},
				},
			},
		})
}
