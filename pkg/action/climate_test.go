package action_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/teslamotors/vehicle-command/pkg/action"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/carserver"
)

var _ = Describe("Climate", func() {
	Describe("SetPreconditioningMax", func() {
		It("returns correct preconditioning settings", func() {
			enabled := true
			manualOverride := true
			action := action.SetPreconditioningMax(enabled, manualOverride)
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetHvacSetPreconditioningMaxAction()).ToNot(BeNil())
			Expect(action.VehicleAction.GetHvacSetPreconditioningMaxAction().On).To(Equal(enabled))
			Expect(action.VehicleAction.GetHvacSetPreconditioningMaxAction().ManualOverride).To(Equal(manualOverride))
		})
	})

	Describe("ClimateOn", func() {
		It("retucts enable climate control system", func() {
			action := action.ClimateOn()
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetHvacAutoAction()).ToNot(BeNil())
			Expect(action.VehicleAction.GetHvacAutoAction().PowerOn).To(BeTrue())
		})
	})

	Describe("ClimateOff", func() {
		It("returns disable climate control system", func() {
			action := action.ClimateOff()
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetHvacAutoAction()).ToNot(BeNil())
			Expect(action.VehicleAction.GetHvacAutoAction().PowerOn).To(BeFalse())
		})
	})

	Describe("AutoSeatAndClimate", func() {
		It("returns correct auto seat and climate settings", func() {
			positions := []action.SeatPosition{action.SeatFrontLeft, action.SeatFrontRight}
			enabled := true
			action := action.AutoSeatAndClimate(positions, enabled)
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetAutoSeatClimateAction()).ToNot(BeNil())
			Expect(action.VehicleAction.GetAutoSeatClimateAction().Carseat).To(HaveLen(len(positions)))
			for _, seat := range action.VehicleAction.GetAutoSeatClimateAction().Carseat {
				Expect(seat.On).To(Equal(enabled))
			}
		})
	})

	Describe("ChangeClimateTemp", func() {
		It("returns correct climate temperature settings", func() {
			driverCelsius := float32(22.0)
			passengerCelsius := float32(22.0)
			action := action.ChangeClimateTemp(driverCelsius, passengerCelsius)
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetHvacTemperatureAdjustmentAction()).ToNot(BeNil())
			Expect(action.VehicleAction.GetHvacTemperatureAdjustmentAction().DriverTempCelsius).To(Equal(driverCelsius))
			Expect(action.VehicleAction.GetHvacTemperatureAdjustmentAction().PassengerTempCelsius).To(Equal(passengerCelsius))
		})
	})

	Describe("SetSeatHeater", func() {
		It("returns correct seat heater settings", func() {
			levels := map[action.SeatPosition]action.Level{
				action.SeatFrontLeft:  action.LevelHigh,
				action.SeatFrontRight: action.LevelHigh,
			}
			action := action.SetSeatHeater(levels)
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetHvacSeatHeaterActions()).ToNot(BeNil())
			Expect(action.VehicleAction.GetHvacSeatHeaterActions().HvacSeatHeaterAction).To(HaveLen(len(levels)))
			for _, action := range action.VehicleAction.GetHvacSeatHeaterActions().HvacSeatHeaterAction {
				Expect(action.SeatHeaterLevel).ToNot(BeNil())
			}
		})
	})

	Describe("SetSteeringWheelHeater", func() {
		It("returns change steering wheel heater state", func() {
			enabled := true
			action := action.SetSteeringWheelHeater(enabled)
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetHvacSteeringWheelHeaterAction()).ToNot(BeNil())
			Expect(action.VehicleAction.GetHvacSteeringWheelHeaterAction().PowerOn).To(Equal(enabled))
		})
	})

	Describe("SetCabinOverheatProtectionTemperature", func() {
		It("returns correct cabin overheat protection temperature settings", func() {
			level := action.LevelHigh
			action := action.SetCabinOverheatProtectionTemperature(level)
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).To(BeAssignableToTypeOf(&carserver.VehicleAction_SetCopTempAction{}))
			Expect(action.VehicleAction.GetVehicleActionMsg().(*carserver.VehicleAction_SetCopTempAction).SetCopTempAction.CopActivationTemp).To(Equal(carserver.ClimateState_CopActivationTemp(level)))
		})
	})

	Describe("SetBioweaponDefenseMode", func() {
		It("returns correct bioweapon defense mode settings", func() {
			enabled := true
			manualOverride := false
			action := action.SetBioweaponDefenseMode(enabled, manualOverride)
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).To(BeAssignableToTypeOf(&carserver.VehicleAction_HvacBioweaponModeAction{}))
			hvacBioweaponModeAction := action.VehicleAction.GetVehicleActionMsg().(*carserver.VehicleAction_HvacBioweaponModeAction).HvacBioweaponModeAction
			Expect(hvacBioweaponModeAction.On).To(Equal(enabled))
			Expect(hvacBioweaponModeAction.ManualOverride).To(Equal(manualOverride))
		})
	})
})
