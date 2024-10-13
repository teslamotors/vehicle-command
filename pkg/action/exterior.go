package action

import "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/carserver"

// HonkHorn honks the vehicle's horn.
func HonkHorn() *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_VehicleControlHonkHornAction{
				VehicleControlHonkHornAction: &carserver.VehicleControlHonkHornAction{},
			},
		},
	}
}

// FlashLights flashes the vehicle's exterior lights.
func FlashLights() *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_VehicleControlFlashLightsAction{
				VehicleControlFlashLightsAction: &carserver.VehicleControlFlashLightsAction{},
			},
		},
	}
}

// ChangeSunroofState changes the state of the sunroof on supported vehicles.
func ChangeSunroofState(sunroofLevel int32) *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_VehicleControlSunroofOpenCloseAction{
				VehicleControlSunroofOpenCloseAction: &carserver.VehicleControlSunroofOpenCloseAction{
					SunroofLevel: &carserver.VehicleControlSunroofOpenCloseAction_AbsoluteLevel{
						AbsoluteLevel: sunroofLevel,
					},
				},
			},
		},
	}
}

// CloseWindows closes the windows on the vehicle.
func CloseWindows() *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_VehicleControlWindowAction{
				VehicleControlWindowAction: &carserver.VehicleControlWindowAction{
					Action: &carserver.VehicleControlWindowAction_Close{
						Close: &carserver.Void{},
					},
				},
			},
		},
	}
}

// VentWindows cracks the windows on the vehicle.
func VentWindows() *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_VehicleControlWindowAction{
				VehicleControlWindowAction: &carserver.VehicleControlWindowAction{
					Action: &carserver.VehicleControlWindowAction_Vent{
						Vent: &carserver.Void{},
					},
				},
			},
		},
	}
}
