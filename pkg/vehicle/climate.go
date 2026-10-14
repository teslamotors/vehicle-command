package vehicle

import (
	"context"
	"fmt"

	carserver "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/carserver"
)

// SetSeatCooler sets seat cooling via published HvacSeatCoolerActions
// (VehicleAction field 49). tesla-http-proxy maps remote_seat_cooler_request
// onto this method. HTTP 501 Unauthorized from Tesla signed_command is Fleet
// API partner/region/OAuth allowlist, not a missing handler. See
// teslamotors/vehicle-command#383.
func (v *Vehicle) SetSeatCooler(ctx context.Context, level Level, seat SeatPosition) error {
	// The protobuf index starts at 0 for unknown, we want to start with 0 for off
	seatMap := map[SeatPosition]carserver.HvacSeatCoolerActions_HvacSeatCoolerPosition_E{
		SeatFrontLeft:  carserver.HvacSeatCoolerActions_HvacSeatCoolerPosition_FrontLeft,
		SeatFrontRight: carserver.HvacSeatCoolerActions_HvacSeatCoolerPosition_FrontRight,
	}
	protoSeat, ok := seatMap[seat]
	if !ok {
		return fmt.Errorf("invalid seat position")
	}
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_HvacSeatCoolerActions{
					HvacSeatCoolerActions: &carserver.HvacSeatCoolerActions{
						HvacSeatCoolerAction: []*carserver.HvacSeatCoolerActions_HvacSeatCoolerAction{
							{
								SeatCoolerLevel: carserver.HvacSeatCoolerActions_HvacSeatCoolerLevel_E(level + 1),
								SeatPosition:    protoSeat,
							},
						},
					},
				},
			},
		})
}

// SetClimatePower turns climate control on or off via HvacAutoAction.power_on
// (Fleet API auto_conditioning_start / auto_conditioning_stop).
//
// Despite the protobuf name, this is climate power, not the in-car Auto vs
// Manual HVAC toggle. Tesla has not published a VehicleAction for that mode.
// Callers that ask for Auto vs Manual should use
// protocol.ErrHvacAutoModeNotInProtocol instead of guessing a field number.
// See teslamotors/vehicle-command#283.
//
// manualOverride is the published HvacAutoAction.manual_override bit: a
// low-SOC override, the same pattern as SetPreconditioningMax,
// SetBioweaponDefenseMode, and SetClimateKeeperMode. It is not Auto vs
// Manual HVAC mode.
func (v *Vehicle) SetClimatePower(ctx context.Context, on, manualOverride bool) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_HvacAutoAction{
					HvacAutoAction: &carserver.HvacAutoAction{
						PowerOn:        on,
						ManualOverride: manualOverride,
					},
				},
			},
		})
}

// ClimateOn turns climate control on. See SetClimatePower.
func (v *Vehicle) ClimateOn(ctx context.Context) error {
	return v.SetClimatePower(ctx, true, false)
}

// ClimateOff turns climate control off. See SetClimatePower.
func (v *Vehicle) ClimateOff(ctx context.Context) error {
	return v.SetClimatePower(ctx, false, false)
}

func (v *Vehicle) AutoSeatAndClimate(ctx context.Context, positions []SeatPosition, enabled bool) error {
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
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_AutoSeatClimateAction{
					AutoSeatClimateAction: &carserver.AutoSeatClimateAction{
						Carseat: seats,
					},
				},
			},
		})
}

// ChangeClimateTemp sets the driver and passenger climate setpoints in Celsius.
//
// Firmware applies driver_temp_celsius and passenger_temp_celsius. Proto3
// leaves unset floats at 0, which the vehicle treats as LO. Sending only
// absolute_celsius (or a zone) without those fields therefore drops the
// setpoint to LO. Level TEMP_MIN / TEMP_MAX are LO / HI, not a flag that
// "numeric temps are present"; this method does not send them. See
// teslamotors/vehicle-command#283.
//
// Independent driver and passenger setpoints are this action. Tesla has not
// published a VehicleAction to toggle the in-car climate split/SYNC control,
// and ClimateState has no split boolean. See teslamotors/vehicle-command#386.
func (v *Vehicle) ChangeClimateTemp(ctx context.Context, driverCelsius float32, passengerCelsius float32) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_HvacTemperatureAdjustmentAction{
					HvacTemperatureAdjustmentAction: &carserver.HvacTemperatureAdjustmentAction{
						DriverTempCelsius:    driverCelsius,
						PassengerTempCelsius: passengerCelsius,
					},
				},
			},
		})
}

// The seat positions defined in the protobuf sources are each independent Void messages instead of
// enumerated values. The autogenerated protobuf code doesn't export the interface that lets us
// declare or access an interface that includes them collectively. The following functions allow us
// to expose a single enumerated type to library clients.

func (s SeatPosition) addToHeaterAction(action *carserver.HvacSeatHeaterActions_HvacSeatHeaterAction) {
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

type Level int

const (
	LevelOff Level = iota
	LevelLow
	LevelMed
	LevelHigh
)

func (s Level) addToHeaterAction(action *carserver.HvacSeatHeaterActions_HvacSeatHeaterAction) {
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

// SetSeatHeater sets seat heating via published HvacSeatHeaterActions
// (VehicleAction field 36). tesla-http-proxy maps remote_seat_heater_request
// onto this method (Owner/Fleet JSON "heater" is an alias for
// "seat_position"). HTTP 501 Unauthorized from Tesla signed_command is Fleet
// API partner/region/OAuth allowlist, not a missing handler. See
// teslamotors/vehicle-command#383.
func (v *Vehicle) SetSeatHeater(ctx context.Context, levels map[SeatPosition]Level) error {
	var actions []*carserver.HvacSeatHeaterActions_HvacSeatHeaterAction

	for position, level := range levels {
		action := new(carserver.HvacSeatHeaterActions_HvacSeatHeaterAction)
		level.addToHeaterAction(action)
		position.addToHeaterAction(action)
		actions = append(actions, action)
	}

	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_HvacSeatHeaterActions{
					HvacSeatHeaterActions: &carserver.HvacSeatHeaterActions{
						HvacSeatHeaterAction: actions,
					},
				},
			},
		})
}

func (v *Vehicle) SetSteeringWheelHeater(ctx context.Context, enabled bool) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_HvacSteeringWheelHeaterAction{
					HvacSteeringWheelHeaterAction: &carserver.HvacSteeringWheelHeaterAction{
						PowerOn: enabled,
					},
				},
			},
		})
}

func (v *Vehicle) SetPreconditioningMax(ctx context.Context, enabled bool, manualOverride bool) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_HvacSetPreconditioningMaxAction{
					HvacSetPreconditioningMaxAction: &carserver.HvacSetPreconditioningMaxAction{
						On:             enabled,
						ManualOverride: manualOverride,
					},
				},
			},
		})
}

func (v *Vehicle) SetBioweaponDefenseMode(ctx context.Context, enabled bool, manualOverride bool) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_HvacBioweaponModeAction{
					HvacBioweaponModeAction: &carserver.HvacBioweaponModeAction{
						On:             enabled,
						ManualOverride: manualOverride,
					},
				},
			},
		})

}

// SetCabinOverheatProtection enables or disables cabin overheat protection.
// fanOnly uses the fan without A/C when the vehicle supports it.
//
// This setting is independent of scheduled charging. On some Intel-MCU Model S
// vehicles, leaving cabin overheat on can prevent the sleep/wake cycle the
// scheduler needs; this method does not couple the two. See
// teslamotors/vehicle-command#342.
func (v *Vehicle) SetCabinOverheatProtection(ctx context.Context, enabled bool, fanOnly bool) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_SetCabinOverheatProtectionAction{
					SetCabinOverheatProtectionAction: &carserver.SetCabinOverheatProtectionAction{
						On:      enabled,
						FanOnly: fanOnly,
					},
				},
			},
		})
}

func (v *Vehicle) SetCabinOverheatProtectionTemperature(ctx context.Context, level Level) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_SetCopTempAction{
					SetCopTempAction: &carserver.SetCopTempAction{
						CopActivationTemp: carserver.ClimateState_CopActivationTemp(level),
					},
				},
			},
		})
}

type ClimateKeeperMode = carserver.HvacClimateKeeperAction_ClimateKeeperAction_E

const (
	ClimateKeeperModeOff  = carserver.HvacClimateKeeperAction_ClimateKeeperAction_Off
	ClimateKeeperModeOn   = carserver.HvacClimateKeeperAction_ClimateKeeperAction_On
	ClimateKeeperModeDog  = carserver.HvacClimateKeeperAction_ClimateKeeperAction_Dog
	ClimateKeeperModeCamp = carserver.HvacClimateKeeperAction_ClimateKeeperAction_Camp
)

// SetClimateKeeperMode sends HvacClimateKeeperAction (Off/On/Dog/Camp).
// Firmware may refuse Dog/Camp with NominalError "cpd_enabled" (Child
// Presence Detection occupancy). That is not the in-car Child Left Alone
// Detection setting. override is a low-SOC bit, not a CPD bypass
// (teslamotors/vehicle-command#437). This method still delivers the
// published action. See teslamotors/vehicle-command#509.
func (v *Vehicle) SetClimateKeeperMode(ctx context.Context, mode ClimateKeeperMode, override bool) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_HvacClimateKeeperAction{
					HvacClimateKeeperAction: &carserver.HvacClimateKeeperAction{
						ClimateKeeperAction: mode,
						ManualOverride:      override,
					},
				},
			},
		})
}
