package action_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/teslamotors/vehicle-command/pkg/action"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/vcsec"
)

var _ = Describe("Closure", func() {
	Describe("ActuateTrunk", func() {
		It("returns with correct trunk action", func() {
			action := action.ActuateTrunk()
			Expect(action).ToNot(BeNil())
			Expect(action.SubMessage).ToNot(BeNil())
			Expect(action.SubMessage).To(BeAssignableToTypeOf(&vcsec.UnsignedMessage_ClosureMoveRequest{}))
			moveRequest := action.SubMessage.(*vcsec.UnsignedMessage_ClosureMoveRequest).ClosureMoveRequest
			Expect(moveRequest).ToNot(BeNil())
			Expect(moveRequest.RearTrunk).To(Equal(vcsec.ClosureMoveType_E_CLOSURE_MOVE_TYPE_MOVE))
		})
	})

	Describe("OpenTrunk", func() {
		It("returns with correct trunk action", func() {
			action := action.OpenTrunk()
			Expect(action).ToNot(BeNil())
			Expect(action.SubMessage).ToNot(BeNil())
			Expect(action.SubMessage).To(BeAssignableToTypeOf(&vcsec.UnsignedMessage_ClosureMoveRequest{}))
			moveRequest := action.SubMessage.(*vcsec.UnsignedMessage_ClosureMoveRequest).ClosureMoveRequest
			Expect(moveRequest).ToNot(BeNil())
			Expect(action.SubMessage.(*vcsec.UnsignedMessage_ClosureMoveRequest).ClosureMoveRequest).ToNot(BeNil())
			Expect(action.SubMessage.(*vcsec.UnsignedMessage_ClosureMoveRequest).ClosureMoveRequest.RearTrunk).To(Equal(vcsec.ClosureMoveType_E_CLOSURE_MOVE_TYPE_OPEN))
		})
	})

	Describe("CloseTrunk", func() {
		It("returns with correct trunk action", func() {
			action := action.CloseTrunk()
			Expect(action).ToNot(BeNil())
			Expect(action.SubMessage).ToNot(BeNil())
			Expect(action.SubMessage).To(BeAssignableToTypeOf(&vcsec.UnsignedMessage_ClosureMoveRequest{}))
			moveRequest := action.SubMessage.(*vcsec.UnsignedMessage_ClosureMoveRequest).ClosureMoveRequest
			Expect(moveRequest).ToNot(BeNil())
			Expect(moveRequest.RearTrunk).To(Equal(vcsec.ClosureMoveType_E_CLOSURE_MOVE_TYPE_CLOSE))
		})
	})

	Describe("OpenFrunk", func() {
		It("returns with correct frunk action", func() {
			action := action.OpenFrunk()
			Expect(action).ToNot(BeNil())
			Expect(action.SubMessage).ToNot(BeNil())
			Expect(action.SubMessage).To(BeAssignableToTypeOf(&vcsec.UnsignedMessage_ClosureMoveRequest{}))
			moveRequest := action.SubMessage.(*vcsec.UnsignedMessage_ClosureMoveRequest).ClosureMoveRequest
			Expect(moveRequest).ToNot(BeNil())
			Expect(moveRequest.FrontTrunk).To(Equal(vcsec.ClosureMoveType_E_CLOSURE_MOVE_TYPE_MOVE))
		})
	})

	Describe("OpenTonneau", func() {
		It("returns with correct tonneau action", func() {
			action := action.OpenTonneau()
			Expect(action).ToNot(BeNil())
			Expect(action.SubMessage).ToNot(BeNil())
			Expect(action.SubMessage).To(BeAssignableToTypeOf(&vcsec.UnsignedMessage_ClosureMoveRequest{}))
			moveRequest := action.SubMessage.(*vcsec.UnsignedMessage_ClosureMoveRequest).ClosureMoveRequest
			Expect(moveRequest).ToNot(BeNil())
			Expect(moveRequest.Tonneau).To(Equal(vcsec.ClosureMoveType_E_CLOSURE_MOVE_TYPE_OPEN))
		})
	})

	Describe("CloseTonneau", func() {
		It("returns with correct tonneau action", func() {
			action := action.CloseTonneau()
			Expect(action).ToNot(BeNil())
			Expect(action.SubMessage).ToNot(BeNil())
			Expect(action.SubMessage).To(BeAssignableToTypeOf(&vcsec.UnsignedMessage_ClosureMoveRequest{}))
			moveRequest := action.SubMessage.(*vcsec.UnsignedMessage_ClosureMoveRequest).ClosureMoveRequest
			Expect(moveRequest).ToNot(BeNil())
			Expect(moveRequest.Tonneau).To(Equal(vcsec.ClosureMoveType_E_CLOSURE_MOVE_TYPE_CLOSE))
		})
	})

	Describe("StopTonneau", func() {
		It("returns with correct tonneau action", func() {
			action := action.StopTonneau()
			Expect(action).ToNot(BeNil())
			Expect(action.SubMessage).ToNot(BeNil())
			Expect(action.SubMessage).To(BeAssignableToTypeOf(&vcsec.UnsignedMessage_ClosureMoveRequest{}))
			moveRequest := action.SubMessage.(*vcsec.UnsignedMessage_ClosureMoveRequest).ClosureMoveRequest
			Expect(moveRequest).ToNot(BeNil())
			Expect(moveRequest.Tonneau).To(Equal(vcsec.ClosureMoveType_E_CLOSURE_MOVE_TYPE_STOP))
		})
	})
})
