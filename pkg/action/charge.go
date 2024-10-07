package action

import (
	"fmt"
	"time"

	carserver "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/carserver"
)

// ChargingPolicy controls when charging should occur.
type ChargingPolicy int

const (
	ChargingPolicyOff ChargingPolicy = iota
	ChargingPolicyAllDays
	ChargingPolicyWeekdays
)

// AddChargeSchedule adds a charge schedule. Requires firmware version 2024.26 or higher.
func AddChargeSchedule(schedule *carserver.ChargeSchedule) *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_AddChargeScheduleAction{
				AddChargeScheduleAction: schedule,
			},
		},
	}
}

// RemoveChargeSchedule removes a charge schedule by ID. Requires firmware version 2024.26 or higher.
func RemoveChargeSchedule(id uint64) *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_RemoveChargeScheduleAction{
				RemoveChargeScheduleAction: &carserver.RemoveChargeScheduleAction{
					Id: id,
				},
			},
		},
	}
}

// BatchRemoveChargeSchedules removes charge schedules for home, work, and other locations.
// Requires firmware version 2024.26 or higher.
func BatchRemoveChargeSchedules(home, work, other bool) *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_BatchRemoveChargeSchedulesAction{
				BatchRemoveChargeSchedulesAction: &carserver.BatchRemoveChargeSchedulesAction{
					Home:  home,
					Work:  work,
					Other: other,
				},
			},
		},
	}
}

// AddPreconditionSchedule adds a precondition schedule.
// Requires firmware version 2024.26 or higher.
func AddPreconditionSchedule(schedule *carserver.PreconditionSchedule) *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_AddPreconditionScheduleAction{
				AddPreconditionScheduleAction: schedule,
			},
		},
	}
}

// RemovePreconditionSchedule removes a precondition schedule by ID.
// Requires firmware version 2024.26 or higher.
func RemovePreconditionSchedule(id uint64) *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_RemovePreconditionScheduleAction{
				RemovePreconditionScheduleAction: &carserver.RemovePreconditionScheduleAction{
					Id: id,
				},
			},
		},
	}
}

// BatchRemovePreconditionSchedules removes precondition schedules for home, work, and other locations. Requires firmware version 2024.26 or higher.
func BatchRemovePreconditionSchedules(home, work, other bool) *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_BatchRemovePreconditionSchedulesAction{
				BatchRemovePreconditionSchedulesAction: &carserver.BatchRemovePreconditionSchedulesAction{
					Home:  home,
					Work:  work,
					Other: other,
				},
			},
		},
	}
}

// ChangeChargeLimit changes the charge limit.
func ChangeChargeLimit(chargeLimitPercent int32) *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_ChargingSetLimitAction{
				ChargingSetLimitAction: &carserver.ChargingSetLimitAction{
					Percent: chargeLimitPercent,
				},
			},
		},
	}
}

// ChargeStart starts charging.
func ChargeStart() *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_ChargingStartStopAction{
				ChargingStartStopAction: &carserver.ChargingStartStopAction{
					ChargingAction: &carserver.ChargingStartStopAction_Start{
						Start: &carserver.Void{},
					},
				},
			},
		},
	}
}

// ChargeStop stops charging.
func ChargeStop() *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_ChargingStartStopAction{
				ChargingStartStopAction: &carserver.ChargingStartStopAction{
					ChargingAction: &carserver.ChargingStartStopAction_Stop{
						Stop: &carserver.Void{},
					},
				},
			},
		},
	}
}

// ChargeMaxRange starts charging in max range mode.
func ChargeMaxRange() *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_ChargingStartStopAction{
				ChargingStartStopAction: &carserver.ChargingStartStopAction{
					ChargingAction: &carserver.ChargingStartStopAction_StartMaxRange{
						StartMaxRange: &carserver.Void{},
					},
				},
			},
		},
	}
}

// SetChargingAmps sets the desired charging amps.
func SetChargingAmps(amps int32) *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_SetChargingAmpsAction{
				SetChargingAmpsAction: &carserver.SetChargingAmpsAction{
					ChargingAmps: amps,
				},
			},
		},
	}
}

// ChargeStandardRange starts charging in standard range mode.
func ChargeStandardRange() *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_ChargingStartStopAction{
				ChargingStartStopAction: &carserver.ChargingStartStopAction{
					ChargingAction: &carserver.ChargingStartStopAction_StartStandard{
						StartStandard: &carserver.Void{},
					},
				},
			},
		},
	}
}

// OpenChargePort opens the charge port.
func OpenChargePort() *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_ChargePortDoorOpen{
				ChargePortDoorOpen: &carserver.ChargePortDoorOpen{},
			},
		},
	}
}

// CloseChargePort closes the charge port.
func CloseChargePort() *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_ChargePortDoorClose{
				ChargePortDoorClose: &carserver.ChargePortDoorClose{},
			},
		},
	}
}

// ScheduledDeparture tells the vehicle to charge based on an expected departure time.
//
// Set departAt and offPeakEndTime relative to midnight.
func ScheduleDeparture(departAt, offPeakEndTime time.Duration, preconditioning, offpeak ChargingPolicy) (*carserver.Action_VehicleAction, error) {
	if departAt < 0 || departAt > 24*time.Hour {
		return nil, fmt.Errorf("invalid departure time")
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

	return &carserver.Action_VehicleAction{
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
	}, nil
}

// ScheduleCharging controls scheduled charging. To start charging at 2:00 AM every day, for
// example, set timeAfterMidnight to 2*time.Hour.
//
// See the Owner's Manual for more information.
func ScheduleCharging(enabled bool, timeAfterMidnight time.Duration) *carserver.Action_VehicleAction {
	minutesFromMidnight := int32(timeAfterMidnight / time.Minute)
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_ScheduledChargingAction{
				ScheduledChargingAction: &carserver.ScheduledChargingAction{
					Enabled:      enabled,
					ChargingTime: minutesFromMidnight,
				},
			},
		},
	}
}

// ClearScheduledDeparture clears the scheduled departure.
func ClearScheduledDeparture() *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_ScheduledDepartureAction{
				ScheduledDepartureAction: &carserver.ScheduledDepartureAction{
					Enabled: false,
				},
			},
		},
	}
}

// GetNearbyChargingSites gets nearby charging sites.
func GetNearbyChargingSites() *carserver.Action_VehicleAction {
	return &carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_GetNearbyChargingSites{
				GetNearbyChargingSites: &carserver.GetNearbyChargingSites{
					IncludeMetaData: true,
					Radius:          200,
					Count:           10,
				},
			},
		},
	}
}
