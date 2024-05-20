// File implements commands that trigger physical vehicle actions.

package vehicle

import (
	"context"

	carserver "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/carserver"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/vcsec"
)

func (v *Vehicle) ActuateTrunk(ctx context.Context) error {
	return v.executeClosureAction(ctx, vcsec.ClosureMoveType_E_CLOSURE_MOVE_TYPE_MOVE, ClosureTrunk)
}

// OpenTrunk opens the trunk, but note that CloseTrunk is not available on all vehicle types.
func (v *Vehicle) OpenTrunk(ctx context.Context) error {
	return v.executeClosureAction(ctx, vcsec.ClosureMoveType_E_CLOSURE_MOVE_TYPE_MOVE, ClosureTrunk)
}

// CloseTrunk is not available on all vehicle types.
func (v *Vehicle) CloseTrunk(ctx context.Context) error {
	return v.executeClosureAction(ctx, vcsec.ClosureMoveType_E_CLOSURE_MOVE_TYPE_CLOSE, ClosureTrunk)
}

// OpenTrunk opens the frunk. There is no remote way to close the frunk!
func (v *Vehicle) OpenFrunk(ctx context.Context) error {
	return v.executeClosureAction(ctx, vcsec.ClosureMoveType_E_CLOSURE_MOVE_TYPE_MOVE, ClosureFrunk)
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

// OpenTonneau opens a Cybetruck's tonneau. Has no effect on other vehicles.
func (v *Vehicle) OpenTonneau(ctx context.Context) error {
	return v.executeClosureAction(ctx, vcsec.ClosureMoveType_E_CLOSURE_MOVE_TYPE_OPEN, ClosureTonneau)
}

// CloseTonneau closes a Cybetruck's tonneau. Has no effect on other vehicles.
func (v *Vehicle) CloseTonneau(ctx context.Context) error {
	return v.executeClosureAction(ctx, vcsec.ClosureMoveType_E_CLOSURE_MOVE_TYPE_CLOSE, ClosureTonneau)
}

// StopTonneau tells a Cybetruck to stop moving its tonneau. Has no effect on other vehicles.
func (v *Vehicle) StopTonneau(ctx context.Context) error {
	return v.executeClosureAction(ctx, vcsec.ClosureMoveType_E_CLOSURE_MOVE_TYPE_STOP, ClosureTonneau)
}
