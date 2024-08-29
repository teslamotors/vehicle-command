// File implements commands related to vehicle charging.

package vehicle

import (
	"context"
	"fmt"
	"time"

	carserver "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/carserver"
)

type ChargingPolicy int

const (
	ChargingPolicyOff ChargingPolicy = iota
	ChargingPolicyAllDays
	ChargingPolicyWeekdays
)

type ChargeSchedule = carserver.ChargeSchedule

type PreconditionSchedule = carserver.PreconditionSchedule

func (v *Vehicle) AddChargeSchedule(ctx context.Context, schedule *ChargeSchedule) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_AddChargeScheduleAction{
					AddChargeScheduleAction: schedule,
				},
			},
		})
}

func (v *Vehicle) RemoveChargeSchedule(ctx context.Context, id uint64) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_RemoveChargeScheduleAction{
					RemoveChargeScheduleAction: &carserver.RemoveChargeScheduleAction{
						Id: id,
					},
				},
			},
		})
}

func (v *Vehicle) BatchRemoveChargeSchedules(ctx context.Context, home, work, other bool) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_BatchRemoveChargeSchedulesAction{
					BatchRemoveChargeSchedulesAction: &carserver.BatchRemoveChargeSchedulesAction{
						Home:  home,
						Work:  work,
						Other: other,
					},
				},
			},
		})
}

func (v *Vehicle) AddPreconditionSchedule(ctx context.Context, schedule *PreconditionSchedule) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_AddPreconditionScheduleAction{
					AddPreconditionScheduleAction: schedule,
				},
			},
		})
}

func (v *Vehicle) RemovePreconditionSchedule(ctx context.Context, id uint64) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_RemovePreconditionScheduleAction{
					RemovePreconditionScheduleAction: &carserver.RemovePreconditionScheduleAction{
						Id: id,
					},
				},
			},
		})
}

func (v *Vehicle) BatchRemovePreconditionSchedules(ctx context.Context, home, work, other bool) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_BatchRemovePreconditionSchedulesAction{
					BatchRemovePreconditionSchedulesAction: &carserver.BatchRemovePreconditionSchedulesAction{
						Home:  home,
						Work:  work,
						Other: other,
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
