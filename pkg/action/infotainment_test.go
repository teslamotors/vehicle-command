package action_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/teslamotors/vehicle-command/pkg/action"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/carserver"
)

var _ = Describe("Infotainment", func() {
	Describe("Ping", func() {
		It("returns ping action", func() {
			action := action.Ping()
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).To(BeAssignableToTypeOf(&carserver.VehicleAction_Ping{}))
			Expect(action.VehicleAction.GetVehicleActionMsg().(*carserver.VehicleAction_Ping).Ping.PingId).To(Equal(int32(1)))
		})
	})

	Describe("SetVolume", func() {
		It("returns set volume action", func() {
			volume := float32(5)
			action, err := action.SetVolume(volume)
			Expect(err).To(BeNil())
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).To(BeAssignableToTypeOf(&carserver.VehicleAction_MediaUpdateVolume{}))
			mediaUpdateVolume := action.VehicleAction.GetVehicleActionMsg().(*carserver.VehicleAction_MediaUpdateVolume).MediaUpdateVolume
			Expect(mediaUpdateVolume.GetVolumeAbsoluteFloat()).To(Equal(volume))
		})

		It("returns error on invalid volume", func() {
			_, err := action.SetVolume(-1)
			Expect(err).ToNot(BeNil())
		})
	})

	Describe("ToggleMediaPlayback", func() {
		It("returns toggle media playback action", func() {
			action := action.ToggleMediaPlayback()
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).To(BeAssignableToTypeOf(&carserver.VehicleAction_MediaPlayAction{}))
			Expect(action.VehicleAction.GetVehicleActionMsg().(*carserver.VehicleAction_MediaPlayAction).MediaPlayAction).ToNot(BeNil())
		})
	})

	Describe("ScheduleSoftwareUpdate", func() {
		It("returns schedule software update action with correct delay", func() {
			delay := 1 * time.Hour
			action := action.ScheduleSoftwareUpdate(delay)
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).To(BeAssignableToTypeOf(&carserver.VehicleAction_VehicleControlScheduleSoftwareUpdateAction{}))
			Expect(action.VehicleAction.GetVehicleActionMsg().(*carserver.VehicleAction_VehicleControlScheduleSoftwareUpdateAction).VehicleControlScheduleSoftwareUpdateAction.OffsetSec).To(Equal(int32(delay.Seconds())))
		})
	})

	Describe("CancelSoftwareUpdate", func() {
		It("returns cancel software update action", func() {
			action := action.CancelSoftwareUpdate()
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).To(BeAssignableToTypeOf(&carserver.VehicleAction_VehicleControlCancelSoftwareUpdateAction{}))
			Expect(action.VehicleAction.GetVehicleActionMsg().(*carserver.VehicleAction_VehicleControlCancelSoftwareUpdateAction).VehicleControlCancelSoftwareUpdateAction).ToNot(BeNil())
		})
	})

	Describe("SetVehicleName", func() {
		It("returns set vehicle name action", func() {
			name := "Test Vehicle"
			action := action.SetVehicleName(name)
			Expect(action).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).ToNot(BeNil())
			Expect(action.VehicleAction.GetVehicleActionMsg()).To(BeAssignableToTypeOf(&carserver.VehicleAction_SetVehicleNameAction{}))
			Expect(action.VehicleAction.GetVehicleActionMsg().(*carserver.VehicleAction_SetVehicleNameAction).SetVehicleNameAction.VehicleName).To(Equal(name))
		})
	})
})
