package action_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/teslamotors/vehicle-command/pkg/action"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/carserver"
)

var _ = Describe("Charge", func() {
	Describe("AddChargeSchedule", func() {
		It("returns with correct schedule", func() {
			schedule := &carserver.ChargeSchedule{}
			action := action.AddChargeSchedule(schedule)
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetAddChargeScheduleAction()).To(Equal(schedule))
		})
	})

	Describe("RemoveChargeSchedule", func() {
		It("returns with correct ID", func() {
			id := uint64(1)
			action := action.RemoveChargeSchedule(id)
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetRemoveChargeScheduleAction()).ToNot(BeNil())
			Expect(action.VehicleAction.GetRemoveChargeScheduleAction().Id).To(Equal(id))
		})
	})

	Describe("BatchRemoveChargeSchedules", func() {
		It("returns with correct locations", func() {
			home, work, other := true, true, true
			action := action.BatchRemoveChargeSchedules(home, work, other)
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetBatchRemoveChargeSchedulesAction()).ToNot(BeNil())
			Expect(action.VehicleAction.GetBatchRemoveChargeSchedulesAction().Home).To(Equal(home))
			Expect(action.VehicleAction.GetBatchRemoveChargeSchedulesAction().Work).To(Equal(work))
			Expect(action.VehicleAction.GetBatchRemoveChargeSchedulesAction().Other).To(Equal(other))
		})
	})

	Describe("AddPreconditionSchedule", func() {
		It("returns with correct schedule", func() {
			schedule := &carserver.PreconditionSchedule{}
			action := action.AddPreconditionSchedule(schedule)
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetAddPreconditionScheduleAction()).To(Equal(schedule))
		})
	})

	Describe("RemovePreconditionSchedule", func() {
		It("returns with correct ID", func() {
			id := uint64(1)
			action := action.RemovePreconditionSchedule(id)
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetRemovePreconditionScheduleAction()).ToNot(BeNil())
			Expect(action.VehicleAction.GetRemovePreconditionScheduleAction().Id).To(Equal(id))
		})
	})

	Describe("BatchRemovePreconditionSchedules", func() {
		It("returns with correct locations", func() {
			home, work, other := true, true, true
			action := action.BatchRemovePreconditionSchedules(home, work, other)
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetBatchRemovePreconditionSchedulesAction()).ToNot(BeNil())
			Expect(action.VehicleAction.GetBatchRemovePreconditionSchedulesAction().Home).To(Equal(home))
			Expect(action.VehicleAction.GetBatchRemovePreconditionSchedulesAction().Work).To(Equal(work))
			Expect(action.VehicleAction.GetBatchRemovePreconditionSchedulesAction().Other).To(Equal(other))
		})
	})

	Describe("ChangeChargeLimit", func() {
		It("returns with correct charge limit", func() {
			chargeLimitPercent := int32(80)
			action := action.ChangeChargeLimit(chargeLimitPercent)
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetChargingSetLimitAction()).ToNot(BeNil())
			Expect(action.VehicleAction.GetChargingSetLimitAction().Percent).To(Equal(chargeLimitPercent))
		})
	})

	Describe("ChargeStart", func() {
		It("returns start charging", func() {
			action := action.ChargeStart()
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetChargingStartStopAction()).ToNot(BeNil())
			Expect(action.VehicleAction.GetChargingStartStopAction().ChargingAction).To(BeAssignableToTypeOf(&carserver.ChargingStartStopAction_Start{}))
		})
	})

	Describe("ChargeStop", func() {
		It("returns stop charging", func() {
			action := action.ChargeStop()
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetChargingStartStopAction()).ToNot(BeNil())
			Expect(action.VehicleAction.GetChargingStartStopAction().ChargingAction).To(BeAssignableToTypeOf(&carserver.ChargingStartStopAction_Stop{}))
		})
	})

	Describe("ChargeMaxRange", func() {
		It("returns start charging in max range mode", func() {
			action := action.ChargeMaxRange()
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetChargingStartStopAction()).ToNot(BeNil())
			Expect(action.VehicleAction.GetChargingStartStopAction().GetChargingAction()).To(BeAssignableToTypeOf(&carserver.ChargingStartStopAction_StartMaxRange{}))
		})
	})

	Describe("SetChargingAmps", func() {
		It("returns correct amps", func() {
			amps := int32(32)
			action := action.SetChargingAmps(amps)
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetSetChargingAmpsAction()).ToNot(BeNil())
			Expect(action.VehicleAction.GetSetChargingAmpsAction().ChargingAmps).To(Equal(amps))
		})
	})

	Describe("ChargeStandardRange", func() {
		It("returns start charging in standard range mode", func() {
			action := action.ChargeStandardRange()
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetChargingStartStopAction()).ToNot(BeNil())
			Expect(action.VehicleAction.GetChargingStartStopAction().GetChargingAction()).To(BeAssignableToTypeOf(&carserver.ChargingStartStopAction_StartStandard{}))
		})
	})

	Describe("OpenChargePort", func() {
		It("returns open charge port", func() {
			action := action.OpenChargePort()
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetChargePortDoorOpen()).ToNot(BeNil())
		})
	})

	Describe("CloseChargePort", func() {
		It("returns close charge port", func() {
			action := action.CloseChargePort()
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetChargePortDoorClose()).To(BeAssignableToTypeOf(&carserver.ChargePortDoorClose{})) // Change expected type
		})
	})

	Describe("ScheduleDeparture", func() {
		It("returns the correct departure time", func() {
			departAt := 2 * time.Hour
			offPeakEndTime := 6 * time.Hour
			preconditioning := action.ChargingPolicyAllDays
			offpeak := action.ChargingPolicyAllDays
			action, err := action.ScheduleDeparture(departAt, offPeakEndTime, preconditioning, offpeak)
			Expect(err).To(BeNil())
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetScheduledDepartureAction()).ToNot(BeNil())
			Expect(action.VehicleAction.GetScheduledDepartureAction().DepartureTime).To(Equal(int32(departAt / time.Minute)))
		})

		It("handles weekdays", func() {
			departAt := 2 * time.Hour
			offPeakEndTime := 6 * time.Hour
			preconditioning := action.ChargingPolicyWeekdays
			offpeak := action.ChargingPolicyWeekdays
			action, err := action.ScheduleDeparture(departAt, offPeakEndTime, preconditioning, offpeak)
			Expect(err).To(BeNil())
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetScheduledDepartureAction()).ToNot(BeNil())
			Expect(action.VehicleAction.GetScheduledDepartureAction().DepartureTime).To(Equal(int32(departAt / time.Minute)))
		})

		It("rejects invalid departure times", func() {
			departAt := -1 * time.Hour
			offPeakEndTime := 6 * time.Hour
			preconditioning := action.ChargingPolicyAllDays
			offpeak := action.ChargingPolicyAllDays
			_, err := action.ScheduleDeparture(departAt, offPeakEndTime, preconditioning, offpeak)
			Expect(err).ToNot(BeNil())
		})
	})

	Describe("ScheduleCharging", func() {
		It("returns with correct charging time", func() {
			enabled := true
			timeAfterMidnight := 2 * time.Hour
			action := action.ScheduleCharging(enabled, timeAfterMidnight)
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetScheduledChargingAction()).ToNot(BeNil())
			Expect(action.VehicleAction.GetScheduledChargingAction().Enabled).To(Equal(enabled))
			Expect(action.VehicleAction.GetScheduledChargingAction().ChargingTime).To(Equal(int32(timeAfterMidnight / time.Minute)))
		})
	})

	Describe("ClearScheduledDeparture", func() {
		It("returns scheduled departure", func() {
			action := action.ClearScheduledDeparture()
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetScheduledDepartureAction()).ToNot(BeNil())
			Expect(action.VehicleAction.GetScheduledDepartureAction().Enabled).To(Equal(false))
		})
	})
	Describe("GetNearbyChargingSites", func() {
		It("returns nearby charging sites", func() {
			action := action.GetNearbyChargingSites()
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetGetNearbyChargingSites()).ToNot(BeNil())
			Expect(action.VehicleAction.GetGetNearbyChargingSites().GetCount()).To(Equal(int32(10)))
			Expect(action.VehicleAction.GetGetNearbyChargingSites().GetRadius()).To(Equal(int32(200)))
			Expect(action.VehicleAction.GetGetNearbyChargingSites().GetIncludeMetaData()).To(Equal(true))
		})
	})
})
