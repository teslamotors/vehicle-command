package action

import "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/vcsec"

// Closure represents a part of the vehicle that opens and closes.
type Closure string

const (
	ClosureTrunk   Closure = "trunk"
	ClosureFrunk   Closure = "frunk"
	ClosureTonneau Closure = "tonneau"
)

// ActuateTrunk opens/closes the trunk state. Note that closing is not available on all vehicles.
func ActuateTrunk() *vcsec.UnsignedMessage {
	return buildClosureAction(vcsec.ClosureMoveType_E_CLOSURE_MOVE_TYPE_MOVE, ClosureTrunk)
}

// OpenTrunk opens the trunk, but note that CloseTrunk is not available on all vehicle types.
func OpenTrunk() *vcsec.UnsignedMessage {
	return buildClosureAction(vcsec.ClosureMoveType_E_CLOSURE_MOVE_TYPE_OPEN, ClosureTrunk)
}

// CloseTrunk is not available on all vehicle types.
func CloseTrunk() *vcsec.UnsignedMessage {
	return buildClosureAction(vcsec.ClosureMoveType_E_CLOSURE_MOVE_TYPE_CLOSE, ClosureTrunk)
}

// OpenFrunk opens the frunk. There is no remote way to close the frunk!
func OpenFrunk() *vcsec.UnsignedMessage {
	return buildClosureAction(vcsec.ClosureMoveType_E_CLOSURE_MOVE_TYPE_MOVE, ClosureFrunk)
}

// OpenTonneau opens a Cybetruck's tonneau. Has no effect on other vehicles.
func OpenTonneau() *vcsec.UnsignedMessage {
	return buildClosureAction(vcsec.ClosureMoveType_E_CLOSURE_MOVE_TYPE_OPEN, ClosureTonneau)
}

// CloseTonneau closes a Cybetruck's tonneau. Has no effect on other vehicles.
func CloseTonneau() *vcsec.UnsignedMessage {
	return buildClosureAction(vcsec.ClosureMoveType_E_CLOSURE_MOVE_TYPE_CLOSE, ClosureTonneau)
}

// StopTonneau tells a Cybetruck to stop moving its tonneau. Has no effect on other vehicles.
func StopTonneau() *vcsec.UnsignedMessage {
	return buildClosureAction(vcsec.ClosureMoveType_E_CLOSURE_MOVE_TYPE_STOP, ClosureTonneau)
}

func buildClosureAction(action vcsec.ClosureMoveType_E, closure Closure) *vcsec.UnsignedMessage {
	// Not all actions are meaningful for all closures. Exported methods restrict combinations.
	var request vcsec.ClosureMoveRequest
	switch closure {
	case ClosureTrunk:
		request.RearTrunk = action
	case ClosureFrunk:
		request.FrontTrunk = action
	case ClosureTonneau:
		request.Tonneau = action
	}

	return &vcsec.UnsignedMessage{
		SubMessage: &vcsec.UnsignedMessage_ClosureMoveRequest{
			ClosureMoveRequest: &request,
		},
	}
}
