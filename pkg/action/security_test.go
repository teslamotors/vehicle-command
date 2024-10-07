package action_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/teslamotors/vehicle-command/pkg/action"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/carserver"
)

var _ = Describe("Security Actions", func() {
	Describe("SetValetMode", func() {
		It("returns set valet mode action", func() {
			action := action.SetValetMode(true, "valetPassword")
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).To(BeAssignableToTypeOf(&carserver.VehicleAction_VehicleControlSetValetModeAction{}))
			valetModeAction := action.VehicleAction.GetVehicleActionMsg().(*carserver.VehicleAction_VehicleControlSetValetModeAction)
			Expect(valetModeAction.VehicleControlSetValetModeAction.On).To(Equal(true))
			Expect(valetModeAction.VehicleControlSetValetModeAction.Password).To(Equal("valetPassword"))
		})
	})

	Describe("ResetValetPin", func() {
		It("returns reset valet PIN action", func() {
			action := action.ResetValetPin()
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).To(BeAssignableToTypeOf(&carserver.VehicleAction_VehicleControlResetValetPinAction{}))
		})
	})

	Describe("ResetPIN", func() {
		It("returns reset PIN action", func() {
			action := action.ResetPIN()
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).To(BeAssignableToTypeOf(&carserver.VehicleAction_VehicleControlResetPinToDriveAction{}))
		})
	})

	Describe("ActivateSpeedLimit", func() {
		It("returns activating speed limit action", func() {
			action := action.ActivateSpeedLimit("speedLimitPin")
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).To(BeAssignableToTypeOf(&carserver.VehicleAction_DrivingSpeedLimitAction{}))
			speedLimitAction := action.VehicleAction.GetVehicleActionMsg().(*carserver.VehicleAction_DrivingSpeedLimitAction)
			Expect(speedLimitAction.DrivingSpeedLimitAction.Activate).To(Equal(true))
			Expect(speedLimitAction.DrivingSpeedLimitAction.Pin).To(Equal("speedLimitPin"))
		})
	})

	Describe("DeactivateSpeedLimit", func() {
		It("returns deactivate speed limit action", func() {
			action := action.DeactivateSpeedLimit("speedLimitPin")
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).To(BeAssignableToTypeOf(&carserver.VehicleAction_DrivingSpeedLimitAction{}))
			speedLimitAction := action.VehicleAction.GetVehicleActionMsg().(*carserver.VehicleAction_DrivingSpeedLimitAction)
			Expect(speedLimitAction.DrivingSpeedLimitAction.Activate).To(Equal(false))
			Expect(speedLimitAction.DrivingSpeedLimitAction.Pin).To(Equal("speedLimitPin"))
		})
	})

	Describe("SpeedLimitSetLimitMPH", func() {
		It("returns set speed limit MPH action", func() {
			action := action.SpeedLimitSetLimitMPH(65)
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).To(BeAssignableToTypeOf(&carserver.VehicleAction_DrivingSetSpeedLimitAction{}))
			speedLimitAction := action.VehicleAction.GetVehicleActionMsg().(*carserver.VehicleAction_DrivingSetSpeedLimitAction)
			Expect(speedLimitAction.DrivingSetSpeedLimitAction.LimitMph).To(Equal(float64(65)))
		})
	})

	Describe("ClearSpeedLimitPIN", func() {
		It("returns clear speed limit PIN action", func() {
			action := action.ClearSpeedLimitPIN("speedLimitPin")
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).To(BeAssignableToTypeOf(&carserver.VehicleAction_DrivingClearSpeedLimitPinAction{}))
			clearSpeedLimitPINAction := action.VehicleAction.GetVehicleActionMsg().(*carserver.VehicleAction_DrivingClearSpeedLimitPinAction)
			Expect(clearSpeedLimitPINAction.DrivingClearSpeedLimitPinAction.Pin).To(Equal("speedLimitPin"))
		})
	})

	Describe("SetSentryMode", func() {
		It("returns set sentry mode action", func() {
			action := action.SetSentryMode(true)
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).To(BeAssignableToTypeOf(&carserver.VehicleAction_VehicleControlSetSentryModeAction{}))
			sentryModeAction := action.VehicleAction.GetVehicleActionMsg().(*carserver.VehicleAction_VehicleControlSetSentryModeAction)
			Expect(sentryModeAction.VehicleControlSetSentryModeAction.On).To(Equal(true))
		})
	})

	Describe("SetGuestMode", func() {
		It("returns set guest mode action", func() {
			action := action.SetGuestMode(true)
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).To(BeAssignableToTypeOf(&carserver.VehicleAction_GuestModeAction{}))
			guestModeAction := action.VehicleAction.GetVehicleActionMsg().(*carserver.VehicleAction_GuestModeAction)
			Expect(guestModeAction.GuestModeAction.GuestModeActive).To(Equal(true))
		})
	})

	Describe("SetPINToDrive", func() {
		It("returns set PIN to drive action", func() {
			action := action.SetPINToDrive(true, "pin")
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).To(BeAssignableToTypeOf(&carserver.VehicleAction_VehicleControlSetPinToDriveAction{}))
			setPINToDriveAction := action.VehicleAction.GetVehicleActionMsg().(*carserver.VehicleAction_VehicleControlSetPinToDriveAction)
			Expect(setPINToDriveAction.VehicleControlSetPinToDriveAction.On).To(Equal(true))
			Expect(setPINToDriveAction.VehicleControlSetPinToDriveAction.Password).To(Equal("pin"))
		})
	})

	Describe("TriggerHomelink", func() {
		It("returns trigger homelink action", func() {
			action := action.TriggerHomelink(37.7749, -122.4194)
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).To(BeAssignableToTypeOf(&carserver.VehicleAction_VehicleControlTriggerHomelinkAction{}))
			triggerHomelinkAction := action.VehicleAction.GetVehicleActionMsg().(*carserver.VehicleAction_VehicleControlTriggerHomelinkAction)
			Expect(triggerHomelinkAction.VehicleControlTriggerHomelinkAction.Location.Latitude).To(Equal(float32(37.7749)))
			Expect(triggerHomelinkAction.VehicleControlTriggerHomelinkAction.Location.Longitude).To(Equal(float32(-122.4194)))
		})
	})

	Describe("EraseGuestData", func() {
		It("returns erase guest data action", func() {
			action := action.EraseGuestData()
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).To(BeAssignableToTypeOf(&carserver.VehicleAction_EraseUserDataAction{}))
		})
	})
})
