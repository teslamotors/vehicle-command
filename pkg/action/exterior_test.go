package action_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/teslamotors/vehicle-command/pkg/action"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/carserver"
)

var _ = Describe("Actions", func() {
	Describe("HonkHorn", func() {
		It("returns honk action", func() {
			action := action.HonkHorn()
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleControlHonkHornAction()).ToNot(BeNil())
		})
	})

	Describe("FlashLights", func() {
		It("returns flash lights action", func() {
			action := action.FlashLights()
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleControlFlashLightsAction()).ToNot(BeNil())
		})
	})

	Describe("ChangeSunroofState", func() {
		It("returns correct sunroof level", func() {
			sunroofLevel := int32(50)
			action := action.ChangeSunroofState(sunroofLevel)
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleControlSunroofOpenCloseAction()).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleControlSunroofOpenCloseAction().GetSunroofLevel()).To(BeAssignableToTypeOf(&carserver.VehicleControlSunroofOpenCloseAction_AbsoluteLevel{}))
			Expect(action.VehicleAction.GetVehicleControlSunroofOpenCloseAction().GetSunroofLevel().(*carserver.VehicleControlSunroofOpenCloseAction_AbsoluteLevel).AbsoluteLevel).To(Equal(sunroofLevel))
		})
	})

	Describe("CloseWindows", func() {
		It("returns close windows", func() {
			action := action.CloseWindows()
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleControlWindowAction().GetClose()).ToNot(BeNil())
		})
	})

	Describe("VentWindows", func() {
		It("returns vent windows", func() {
			action := action.VentWindows()
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleControlWindowAction().GetVent()).ToNot(BeNil())
		})
	})

	Describe("SetClimateKeeperMode", func() {
		It("returns correct climate keeper mode action", func() {
			override := true
			action := action.SetClimateKeeperMode(action.ClimateKeeperModeCamp, override)
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).To(BeAssignableToTypeOf(&carserver.VehicleAction_HvacClimateKeeperAction{}))
			hvacClimateKeeperAction := action.VehicleAction.GetVehicleActionMsg().(*carserver.VehicleAction_HvacClimateKeeperAction)
			Expect(hvacClimateKeeperAction.HvacClimateKeeperAction.GetManualOverride()).To(Equal(override))
			Expect(hvacClimateKeeperAction.HvacClimateKeeperAction.GetClimateKeeperAction()).To(Equal(carserver.HvacClimateKeeperAction_ClimateKeeperAction_Camp))
		})
	})

	Describe("SetCabinOverheatProtection", func() {
		It("returns set cabin overheat protection action", func() {
			enabled, fanOnly := true, true
			action := action.SetCabinOverheatProtection(enabled, fanOnly)
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).To(BeAssignableToTypeOf(&carserver.VehicleAction_SetCabinOverheatProtectionAction{}))
			setCabinOverheatProtectionAction := action.VehicleAction.GetVehicleActionMsg().(*carserver.VehicleAction_SetCabinOverheatProtectionAction)
			Expect(setCabinOverheatProtectionAction.SetCabinOverheatProtectionAction.On).To(Equal(enabled))
			Expect(setCabinOverheatProtectionAction.SetCabinOverheatProtectionAction.FanOnly).To(Equal(fanOnly))
		})
	})

	Describe("SetSeatCooler", func() {
		It("returns set seat cooler action", func() {
			seat := action.SeatFrontLeft
			level := action.LevelLow
			action, err := action.SetSeatCooler(level, seat)
			Expect(err).ToNot(HaveOccurred())
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).To(BeAssignableToTypeOf(&carserver.VehicleAction_HvacSeatCoolerActions{}))
			hvacSeatCoolerActions := action.VehicleAction.GetVehicleActionMsg().(*carserver.VehicleAction_HvacSeatCoolerActions)
			actions := hvacSeatCoolerActions.HvacSeatCoolerActions.GetHvacSeatCoolerAction()
			Expect(actions).To(HaveLen(1))
			Expect(actions[0].GetSeatPosition()).To(Equal(carserver.HvacSeatCoolerActions_HvacSeatCoolerPosition_FrontLeft))
			Expect(actions[0].GetSeatCoolerLevel()).To(Equal(carserver.HvacSeatCoolerActions_HvacSeatCoolerLevel_Low))
		})

		It("returns error on invalid seat cooler level", func() {
			seat := action.SeatSecondRowCenter
			level := action.LevelLow
			_, err := action.SetSeatCooler(level, seat)
			Expect(err).To(HaveOccurred())
		})
	})
})
