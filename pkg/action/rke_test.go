package action_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/teslamotors/vehicle-command/pkg/action"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/vcsec"
)

var _ = Describe("RKE Actions", func() {
	Describe("AutoSecureVehicle", func() {
		It("returns auto secure vehicle action", func() {
			action := action.AutoSecureVehicle()
			Expect(action).ToNot(BeNil())
			Expect(action.SubMessage).ToNot(BeNil())
			Expect(action.SubMessage).To(BeAssignableToTypeOf(&vcsec.UnsignedMessage_RKEAction{}))
			Expect(action.SubMessage.(*vcsec.UnsignedMessage_RKEAction).RKEAction).To(Equal(vcsec.RKEAction_E_RKE_ACTION_AUTO_SECURE_VEHICLE))
		})
	})

	Describe("Lock", func() {
		It("returns lock vehicle action", func() {
			action := action.Lock()
			Expect(action).ToNot(BeNil())
			Expect(action.SubMessage).ToNot(BeNil())
			Expect(action.SubMessage).To(BeAssignableToTypeOf(&vcsec.UnsignedMessage_RKEAction{}))
			Expect(action.SubMessage.(*vcsec.UnsignedMessage_RKEAction).RKEAction).To(Equal(vcsec.RKEAction_E_RKE_ACTION_LOCK))
		})
	})

	Describe("Unlock", func() {
		It("returns unlock action", func() {
			action := action.Unlock()
			Expect(action).ToNot(BeNil())
			Expect(action.SubMessage).ToNot(BeNil())
			Expect(action.SubMessage).To(BeAssignableToTypeOf(&vcsec.UnsignedMessage_RKEAction{}))
			Expect(action.SubMessage.(*vcsec.UnsignedMessage_RKEAction).RKEAction).To(Equal(vcsec.RKEAction_E_RKE_ACTION_UNLOCK))
		})
	})

	Describe("RemoteDrive", func() {
		It("returns remote drive action", func() {
			action := action.RemoteDrive()
			Expect(action).ToNot(BeNil())
			Expect(action.SubMessage).ToNot(BeNil())
			Expect(action.SubMessage).To(BeAssignableToTypeOf(&vcsec.UnsignedMessage_RKEAction{}))
			Expect(action.SubMessage.(*vcsec.UnsignedMessage_RKEAction).RKEAction).To(Equal(vcsec.RKEAction_E_RKE_ACTION_REMOTE_DRIVE))
		})
	})
})
