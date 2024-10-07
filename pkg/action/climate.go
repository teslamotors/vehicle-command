package action

import (
	"fmt"

	carserver "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/carserver"
)

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

type ClimateKeeperMode = carserver.HvacClimateKeeperAction_ClimateKeeperAction_E

const (
	ClimateKeeperModeOff  = carserver.HvacClimateKeeperAction_ClimateKeeperAction_Off
	ClimateKeeperModeOn   = carserver.HvacClimateKeeperAction_ClimateKeeperAction_On
	ClimateKeeperModeDog  = carserver.HvacClimateKeeperAction_ClimateKeeperAction_Dog
	ClimateKeeperModeCamp = carserver.HvacClimateKeeperAction_ClimateKeeperAction_Camp
)

type Level int

const (
	LevelOff Level = iota
	LevelLow
	LevelMed
	LevelHigh
)

// SetSeatCooler sets seat cooling level.
func SetSeatCooler(level Level, seat SeatPosition) (*carserver.Action_VehicleAction, error) {
	// The protobuf index starts at 0 for unknown, we want to start with 0 for off
	seatMap := map[SeatPosition]carserver.HvacSeatCoolerActions_HvacSeatCoolerPosition_E{
		SeatFrontLeft:  carserver.HvacSeatCoolerActions_HvacSeatCoolerPosition_FrontLeft,
		SeatFrontRight: carserver.HvacSeatCoolerActions_HvacSeatCoolerPosition_FrontRight,
	}
	protoSeat, ok := seatMap[seat]
	if !ok {
		return nil, fmt.Errorf("invalid seat position")
	}
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_HvacSeatCoolerActions{
				HvacSeatCoolerActions: &carserver.HvacSeatCoolerActions{
					HvacSeatCoolerAction: []*carserver.HvacSeatCoolerActions_HvacSeatCoolerAction{
						&carserver.HvacSeatCoolerActions_HvacSeatCoolerAction{
							SeatCoolerLevel: carserver.HvacSeatCoolerActions_HvacSeatCoolerLevel_E(level + 1),
							SeatPosition:    protoSeat,
						},
					},
				},
			},
		},
	}, nil
}

// ClimateOn turns on the climate control system.
func ClimateOn() *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_HvacAutoAction{
				HvacAutoAction: &carserver.HvacAutoAction{
					PowerOn: true,
				},
			},
		},
	}
}

// ClimateOff turns off the climate control system.
func ClimateOff() *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_HvacAutoAction{
				HvacAutoAction: &carserver.HvacAutoAction{
					PowerOn: false,
				},
			},
		},
	}
}

// AutoSeatAndClimate turns on or off automatic climate control for the specified seats.
func AutoSeatAndClimate(positions []SeatPosition, enabled bool) *carserver.Action_VehicleAction {
	lookup := map[SeatPosition]carserver.AutoSeatClimateAction_AutoSeatPosition_E{
		SeatUnknown:    carserver.AutoSeatClimateAction_AutoSeatPosition_Unknown,
		SeatFrontLeft:  carserver.AutoSeatClimateAction_AutoSeatPosition_FrontLeft,
		SeatFrontRight: carserver.AutoSeatClimateAction_AutoSeatPosition_FrontRight,
	}
	var seats []*carserver.AutoSeatClimateAction_CarSeat
	for _, pos := range positions {
		if protoPos, ok := lookup[pos]; ok {
			seats = append(seats, &carserver.AutoSeatClimateAction_CarSeat{On: enabled, SeatPosition: protoPos})
		}
	}
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_AutoSeatClimateAction{
				AutoSeatClimateAction: &carserver.AutoSeatClimateAction{
					Carseat: seats,
				},
			},
		},
	}
}

// ChangeClimateTemp sets the desired temperature for the climate control system.
func ChangeClimateTemp(driverCelsius float32, passengerCelsius float32) *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_HvacTemperatureAdjustmentAction{
				HvacTemperatureAdjustmentAction: &carserver.HvacTemperatureAdjustmentAction{
					DriverTempCelsius:    driverCelsius,
					PassengerTempCelsius: passengerCelsius,
					Level: &carserver.HvacTemperatureAdjustmentAction_Temperature{
						Type: &carserver.HvacTemperatureAdjustmentAction_Temperature_TEMP_MAX{},
					},
				},
			},
		},
	}
}

func SetSeatHeater(levels map[SeatPosition]Level) *carserver.Action_VehicleAction {
	actions := make([]*carserver.HvacSeatHeaterActions_HvacSeatHeaterAction, 0, len(levels))

	for position, level := range levels {
		action := new(carserver.HvacSeatHeaterActions_HvacSeatHeaterAction)
		level.addToSeatHeaterAction(action)
		position.addToSeatPositionAction(action)
		actions = append(actions, action)
	}

	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_HvacSeatHeaterActions{
				HvacSeatHeaterActions: &carserver.HvacSeatHeaterActions{
					HvacSeatHeaterAction: actions,
				},
			},
		},
	}
}

// SetSteeringWheelHeater turns on or off the steering wheel heater for supported vehicles.
func SetSteeringWheelHeater(enabled bool) *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_HvacSteeringWheelHeaterAction{
				HvacSteeringWheelHeaterAction: &carserver.HvacSteeringWheelHeaterAction{
					PowerOn: enabled,
				},
			},
		},
	}
}

// SetPreconditioningMax turns on or off preconditioning for supported vehicles.
func SetPreconditioningMax(enabled bool, manualOverride bool) *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_HvacSetPreconditioningMaxAction{
				HvacSetPreconditioningMaxAction: &carserver.HvacSetPreconditioningMaxAction{
					On:             enabled,
					ManualOverride: manualOverride,
				},
			},
		},
	}
}

func SetBioweaponDefenseMode(enabled bool, manualOverride bool) *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_HvacBioweaponModeAction{
				HvacBioweaponModeAction: &carserver.HvacBioweaponModeAction{
					On:             enabled,
					ManualOverride: manualOverride,
				},
			},
		},
	}
}

// SetCabinOverheatProtection turns on or off cabin overheat protection for supported vehicles.
func SetCabinOverheatProtection(enabled bool, fanOnly bool) *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_SetCabinOverheatProtectionAction{
				SetCabinOverheatProtectionAction: &carserver.SetCabinOverheatProtectionAction{
					On:      enabled,
					FanOnly: fanOnly,
				},
			},
		},
	}
}

// SetCabinOverheatProtectionTemperature sets the cabin overheat protection activation temperature.
func SetCabinOverheatProtectionTemperature(level Level) *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_SetCopTempAction{
				SetCopTempAction: &carserver.SetCopTempAction{
					CopActivationTemp: carserver.ClimateState_CopActivationTemp(level),
				},
			},
		},
	}
}

// SetClimateKeeperMode sets the climate keeper mode.
func SetClimateKeeperMode(mode ClimateKeeperMode, override bool) *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_HvacClimateKeeperAction{
				HvacClimateKeeperAction: &carserver.HvacClimateKeeperAction{
					ClimateKeeperAction: mode,
					ManualOverride:      override,
				},
			},
		},
	}
}

// The seat positions defined in the protobuf sources are each independent Void messages instead of
// enumerated values. The autogenerated protobuf code doesn't export the interface that lets us
// declare or access an interface that includes them collectively. The following functions allow us
// to expose a single enumerated type to library clients.

func (s SeatPosition) addToSeatPositionAction(action *carserver.HvacSeatHeaterActions_HvacSeatHeaterAction) {
	switch s {
	case SeatFrontLeft:
		action.SeatPosition = &carserver.HvacSeatHeaterActions_HvacSeatHeaterAction_CAR_SEAT_FRONT_LEFT{}
	case SeatFrontRight:
		action.SeatPosition = &carserver.HvacSeatHeaterActions_HvacSeatHeaterAction_CAR_SEAT_FRONT_RIGHT{}
	case SeatSecondRowLeft:
		action.SeatPosition = &carserver.HvacSeatHeaterActions_HvacSeatHeaterAction_CAR_SEAT_REAR_LEFT{}
	case SeatSecondRowLeftBack:
		action.SeatPosition = &carserver.HvacSeatHeaterActions_HvacSeatHeaterAction_CAR_SEAT_REAR_LEFT_BACK{}
	case SeatSecondRowCenter:
		action.SeatPosition = &carserver.HvacSeatHeaterActions_HvacSeatHeaterAction_CAR_SEAT_REAR_CENTER{}
	case SeatSecondRowRight:
		action.SeatPosition = &carserver.HvacSeatHeaterActions_HvacSeatHeaterAction_CAR_SEAT_REAR_RIGHT{}
	case SeatSecondRowRightBack:
		action.SeatPosition = &carserver.HvacSeatHeaterActions_HvacSeatHeaterAction_CAR_SEAT_REAR_RIGHT_BACK{}
	case SeatThirdRowLeft:
		action.SeatPosition = &carserver.HvacSeatHeaterActions_HvacSeatHeaterAction_CAR_SEAT_THIRD_ROW_LEFT{}
	case SeatThirdRowRight:
		action.SeatPosition = &carserver.HvacSeatHeaterActions_HvacSeatHeaterAction_CAR_SEAT_THIRD_ROW_RIGHT{}
	default:
		action.SeatPosition = &carserver.HvacSeatHeaterActions_HvacSeatHeaterAction_CAR_SEAT_UNKNOWN{}
	}
}

func (s Level) addToSeatHeaterAction(action *carserver.HvacSeatHeaterActions_HvacSeatHeaterAction) {
	switch s {
	case LevelOff:
		action.SeatHeaterLevel = &carserver.HvacSeatHeaterActions_HvacSeatHeaterAction_SEAT_HEATER_OFF{}
	case LevelLow:
		action.SeatHeaterLevel = &carserver.HvacSeatHeaterActions_HvacSeatHeaterAction_SEAT_HEATER_LOW{}
	case LevelMed:
		action.SeatHeaterLevel = &carserver.HvacSeatHeaterActions_HvacSeatHeaterAction_SEAT_HEATER_MED{}
	case LevelHigh:
		action.SeatHeaterLevel = &carserver.HvacSeatHeaterActions_HvacSeatHeaterAction_SEAT_HEATER_HIGH{}
	default:
		action.SeatHeaterLevel = &carserver.HvacSeatHeaterActions_HvacSeatHeaterAction_SEAT_HEATER_UNKNOWN{}
	}
}
