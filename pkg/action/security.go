package action

import (
	carserver "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/carserver"
)

// SetValetMode sets the valet mode on or off. If enabling, sets the password.
func SetValetMode(on bool, valetPassword string) *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_VehicleControlSetValetModeAction{
				VehicleControlSetValetModeAction: &carserver.VehicleControlSetValetModeAction{
					On:       on,
					Password: valetPassword,
				},
			},
		},
	}
}

// ResetValetPin resets the valet pin.
func ResetValetPin() *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_VehicleControlResetValetPinAction{
				VehicleControlResetValetPinAction: &carserver.VehicleControlResetValetPinAction{},
			},
		},
	}
}

// ResetPIN clears the saved PIN. You must disable PIN to drive before clearing the PIN. This allows
// setting a new PIN using SetPINToDrive.
func ResetPIN() *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_VehicleControlResetPinToDriveAction{
				VehicleControlResetPinToDriveAction: &carserver.VehicleControlResetPinToDriveAction{},
			},
		},
	}
}

// ActivateSpeedLimit limits the maximum speed of the vehicle. The actual speed limit is set
// using SpeedLimitSetLimitMPH.
func ActivateSpeedLimit(speedLimitPin string) *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_DrivingSpeedLimitAction{
				DrivingSpeedLimitAction: &carserver.DrivingSpeedLimitAction{
					Activate: true,
					Pin:      speedLimitPin,
				},
			},
		},
	}
}

// DeactivateSpeedLimit removes a maximum speed restriction from the vehicle.
func DeactivateSpeedLimit(speedLimitPin string) *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_DrivingSpeedLimitAction{
				DrivingSpeedLimitAction: &carserver.DrivingSpeedLimitAction{
					Activate: false,
					Pin:      speedLimitPin,
				},
			},
		},
	}
}

// SpeedLimitSetLimitMPH sets the speed limit in MPH.
func SpeedLimitSetLimitMPH(speedLimitMPH float64) *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_DrivingSetSpeedLimitAction{
				DrivingSetSpeedLimitAction: &carserver.DrivingSetSpeedLimitAction{
					LimitMph: speedLimitMPH,
				},
			},
		},
	}
}

// ClearSpeedLimitPIN clears the speed limit pin.
func ClearSpeedLimitPIN(speedLimitPin string) *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_DrivingClearSpeedLimitPinAction{
				DrivingClearSpeedLimitPinAction: &carserver.DrivingClearSpeedLimitPinAction{
					Pin: speedLimitPin,
				},
			},
		},
	}
}

// SetSentryMode enables or disables sentry mode.
func SetSentryMode(state bool) *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_VehicleControlSetSentryModeAction{
				VehicleControlSetSentryModeAction: &carserver.VehicleControlSetSentryModeAction{
					On: state,
				},
			},
		},
	}
}

// SetGuestMode enables or disables the vehicle's guest mode.
//
// We recommend users avoid this command unless they are managing a fleet of vehicles and understand
// the implications of enabling the mode. See official API documentation at
// https://developer.tesla.com/docs/fleet-api/endpoints/vehicle-commands#guest-mode
func SetGuestMode(enabled bool) *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_GuestModeAction{
				GuestModeAction: &carserver.VehicleState_GuestMode{
					GuestModeActive: enabled,
				},
			},
		},
	}
}

// SetPINToDrive controls whether the PIN to Drive feature is enabled or not. It is also used to set
// the PIN.

// Once a PIN is set, the vehicle remembers its value even when PIN to Drive is disabled and
// discards any new PIN provided using this method. To change an existing PIN, first call
// v.ResetPIN.
//
// Must be used through Fleet API.
func SetPINToDrive(enabled bool, pin string) *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_VehicleControlSetPinToDriveAction{
				VehicleControlSetPinToDriveAction: &carserver.VehicleControlSetPinToDriveAction{
					On:       enabled,
					Password: pin,
				},
			},
		},
	}
}

// TriggerHomelink triggers homelink at a given coordinate.
func TriggerHomelink(latitude float32, longitude float32) *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_VehicleControlTriggerHomelinkAction{
				VehicleControlTriggerHomelinkAction: &carserver.VehicleControlTriggerHomelinkAction{
					Location: &carserver.LatLong{
						Latitude:  latitude,
						Longitude: longitude,
					},
				},
			},
		},
	}
}

// EraseGuestData erases user data created while in Guest Mode. This command has no effect unless
// the vehicle is currently in Guest Mode.
func EraseGuestData() *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_EraseUserDataAction{},
		},
	}
}
