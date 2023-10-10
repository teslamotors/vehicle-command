// This file implements commands that terminate on the INFOTAINMENT domain.
// This includes media, HVAC, and essentially everything that cannot be
// controlled using a keyfob or NFC card.

package vehicle

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/teslamotors/vehicle-command/pkg/connector"
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

// SetSeatCooler sets seat cooling level.
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
							&carserver.HvacSeatCoolerActions_HvacSeatCoolerAction{
								SeatCoolerLevel: carserver.HvacSeatCoolerActions_HvacSeatCoolerLevel_E(level + 1),
								SeatPosition:    protoSeat,
							},
						},
					},
				},
			},
		})
}

func (v *Vehicle) ClimateOn(ctx context.Context) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_HvacAutoAction{
					HvacAutoAction: &carserver.HvacAutoAction{
						PowerOn: true,
					},
				},
			},
		})
}

func (v *Vehicle) ClimateOff(ctx context.Context) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_HvacAutoAction{
					HvacAutoAction: &carserver.HvacAutoAction{
						PowerOn: false,
					},
				},
			},
		})
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

func (v *Vehicle) ChangeClimateTemp(ctx context.Context, driverCelsius float32, passengerCelsius float32) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
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
		})
}

func (v *Vehicle) ChangeChargeLimit(ctx context.Context, chargeLimitPercent int32) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_ChargingSetLimitAction{
					ChargingSetLimitAction: &carserver.ChargingSetLimitAction{
						Percent: chargeLimitPercent,
					},
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

func (v *Vehicle) ChargeStart(ctx context.Context) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_ChargingStartStopAction{
					ChargingStartStopAction: &carserver.ChargingStartStopAction{
						ChargingAction: &carserver.ChargingStartStopAction_Start{
							Start: &carserver.Void{},
						},
					},
				},
			},
		})
}

func (v *Vehicle) ChargeStop(ctx context.Context) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_ChargingStartStopAction{
					ChargingStartStopAction: &carserver.ChargingStartStopAction{
						ChargingAction: &carserver.ChargingStartStopAction_Stop{
							Stop: &carserver.Void{},
						},
					},
				},
			},
		})
}

func (v *Vehicle) ChargeMaxRange(ctx context.Context) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_ChargingStartStopAction{
					ChargingStartStopAction: &carserver.ChargingStartStopAction{
						ChargingAction: &carserver.ChargingStartStopAction_StartMaxRange{
							StartMaxRange: &carserver.Void{},
						},
					},
				},
			},
		})
}

func (v *Vehicle) SetChargingAmps(ctx context.Context, amps int32) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_SetChargingAmpsAction{
					SetChargingAmpsAction: &carserver.SetChargingAmpsAction{
						ChargingAmps: amps,
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

func (v *Vehicle) ChargeStandardRange(ctx context.Context) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_ChargingStartStopAction{
					ChargingStartStopAction: &carserver.ChargingStartStopAction{
						ChargingAction: &carserver.ChargingStartStopAction_StartStandard{
							StartStandard: &carserver.Void{},
						},
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

func (v *Vehicle) SetValetMode(ctx context.Context, on bool, valetPassword string) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_VehicleControlSetValetModeAction{
					VehicleControlSetValetModeAction: &carserver.VehicleControlSetValetModeAction{
						On:       on,
						Password: valetPassword,
					},
				},
			},
		})
}

func (v *Vehicle) ResetValetPin(ctx context.Context) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_VehicleControlResetValetPinAction{
					VehicleControlResetValetPinAction: &carserver.VehicleControlResetValetPinAction{},
				},
			},
		})
}

// ResetPIN clears the saved PIN. You must disable PIN to drive before clearing the PIN. This allows
// setting a new PIN using SetPINToDrive.
func (v *Vehicle) ResetPIN(ctx context.Context) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_VehicleControlResetPinToDriveAction{
					VehicleControlResetPinToDriveAction: &carserver.VehicleControlResetPinToDriveAction{},
				},
			},
		})
}

type ChargingPolicy int

const (
	ChargingPolicyOff ChargingPolicy = iota
	ChargingPolicyAllDays
	ChargingPolicyWeekdays
)

func (v *Vehicle) ClearScheduledDeparture(ctx context.Context) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_ScheduledDepartureAction{
					ScheduledDepartureAction: &carserver.ScheduledDepartureAction{
						Enabled: false,
					},
				},
			},
		})
}

// ScheduledDeparture tells the vehicle to charge based on an expected departure time.
//
// Set departAt and offPeakEndTime relative to midnight.
func (v *Vehicle) ScheduleDeparture(ctx context.Context, departAt, offPeakEndTime time.Duration, preconditioning, offpeak ChargingPolicy) error {
	if departAt < 0 || departAt > 24*time.Hour {
		return fmt.Errorf("invalid departure time")
	}
	var preconditionProto *carserver.PreconditioningTimes
	switch preconditioning {
	case ChargingPolicyOff:
	case ChargingPolicyAllDays:
		preconditionProto = &carserver.PreconditioningTimes{
			Times: &carserver.PreconditioningTimes_AllWeek{
				AllWeek: &carserver.Void{},
			},
		}
	case ChargingPolicyWeekdays:
		preconditionProto = &carserver.PreconditioningTimes{
			Times: &carserver.PreconditioningTimes_Weekdays{
				Weekdays: &carserver.Void{},
			},
		}
	}

	var offPeakProto *carserver.OffPeakChargingTimes
	switch offpeak {
	case ChargingPolicyOff:
	case ChargingPolicyAllDays:
		offPeakProto = &carserver.OffPeakChargingTimes{
			Times: &carserver.OffPeakChargingTimes_AllWeek{
				AllWeek: &carserver.Void{},
			},
		}
	case ChargingPolicyWeekdays:
		offPeakProto = &carserver.OffPeakChargingTimes{
			Times: &carserver.OffPeakChargingTimes_Weekdays{
				Weekdays: &carserver.Void{},
			},
		}
	}

	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_ScheduledDepartureAction{
					ScheduledDepartureAction: &carserver.ScheduledDepartureAction{
						Enabled:              true,
						DepartureTime:        int32(departAt / time.Minute),
						PreconditioningTimes: preconditionProto,
						OffPeakChargingTimes: offPeakProto,
						OffPeakHoursEndTime:  int32(offPeakEndTime / time.Minute),
					},
				},
			},
		})
}

func (v *Vehicle) ActivateSpeedLimit(ctx context.Context, speedLimitPin string) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_DrivingSpeedLimitAction{
					DrivingSpeedLimitAction: &carserver.DrivingSpeedLimitAction{
						Activate: true,
						Pin:      speedLimitPin,
					},
				},
			},
		})
}

func (v *Vehicle) DeactivateSpeedLimit(ctx context.Context, speedLimitPin string) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_DrivingSpeedLimitAction{
					DrivingSpeedLimitAction: &carserver.DrivingSpeedLimitAction{
						Activate: false,
						Pin:      speedLimitPin,
					},
				},
			},
		})
}

func (v *Vehicle) SpeedLimitSetLimitMPH(ctx context.Context, speedLimitMPH float64) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_DrivingSetSpeedLimitAction{
					DrivingSetSpeedLimitAction: &carserver.DrivingSetSpeedLimitAction{
						LimitMph: speedLimitMPH,
					},
				},
			},
		})
}

func (v *Vehicle) ClearSpeedLimitPIN(ctx context.Context, speedLimitPin string) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_DrivingClearSpeedLimitPinAction{
					DrivingClearSpeedLimitPinAction: &carserver.DrivingClearSpeedLimitPinAction{
						Pin: speedLimitPin,
					},
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

func (v *Vehicle) SetSentryMode(ctx context.Context, state bool) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_VehicleControlSetSentryModeAction{
					VehicleControlSetSentryModeAction: &carserver.VehicleControlSetSentryModeAction{
						On: state,
					},
				},
			},
		})
}

// SetGuestMode enables or disables the vehicle's guest mode.
//
// We recommend users avoid this command unless they are managing a fleet of vehicles and understand
// the implications of enabling the mode. See official API documentation at
// https://developer.tesla.com/docs/fleet-api#guest_mode.
func (v *Vehicle) SetGuestMode(ctx context.Context, enabled bool) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_GuestModeAction{
					GuestModeAction: &carserver.VehicleState_GuestMode{
						GuestModeActive: enabled,
					},
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

func (v *Vehicle) TriggerHomelink(ctx context.Context, latitude float32, longitude float32) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
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

func (v *Vehicle) OpenChargePort(ctx context.Context) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_ChargePortDoorOpen{
					ChargePortDoorOpen: &carserver.ChargePortDoorOpen{},
				},
			},
		})
}

func (v *Vehicle) CloseChargePort(ctx context.Context) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_ChargePortDoorClose{
					ChargePortDoorClose: &carserver.ChargePortDoorClose{},
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

// SetPINToDrive controls whether the PIN to Drive feature is enabled or not. It is also used to set
// the PIN.
//
// Once a PIN is set, the vehicle remembers its value even when PIN to Drive is disabled and
// discards any new PIN provided using this method. To change an existing PIN, first call
// v.ResetPIN.
func (v *Vehicle) SetPINToDrive(ctx context.Context, enabled bool, pin string) error {
	if _, ok := v.conn.(connector.FleetAPIConnector); !ok {
		return protocol.ErrRequiresEncryption
	}

	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_VehicleControlSetPinToDriveAction{
					VehicleControlSetPinToDriveAction: &carserver.VehicleControlSetPinToDriveAction{
						On:       enabled,
						Password: pin,
					},
				},
			},
		})
}

// ScheduleCharging controls scheduled charging. To start charging at 2:00 AM every day, for
// example, set timeAfterMidnight to 2*time.Hour.
//
// See the Owner's Manual for more information.
func (v *Vehicle) ScheduleCharging(ctx context.Context, enabled bool, timeAfterMidnight time.Duration) error {
	minutesFromMidnight := int32(timeAfterMidnight / time.Minute)
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_ScheduledChargingAction{
					ScheduledChargingAction: &carserver.ScheduledChargingAction{
						Enabled:      enabled,
						ChargingTime: minutesFromMidnight,
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
