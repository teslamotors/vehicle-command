package action

import "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/vcsec"

// AutoSecureVehicle secures the vehicle by locking and closing doors and windows.
func AutoSecureVehicle() *vcsec.UnsignedMessage {
	return buildRKEAction(vcsec.RKEAction_E_RKE_ACTION_AUTO_SECURE_VEHICLE)
}

// Lock locks the vehicle.
func Lock() *vcsec.UnsignedMessage {
	return buildRKEAction(vcsec.RKEAction_E_RKE_ACTION_LOCK)
}

// Unlock unlocks the vehicle.
func Unlock() *vcsec.UnsignedMessage {
	return buildRKEAction(vcsec.RKEAction_E_RKE_ACTION_UNLOCK)
}

// RemoteDrive allows the vehicle to be driven.
func RemoteDrive() *vcsec.UnsignedMessage {
	return buildRKEAction(vcsec.RKEAction_E_RKE_ACTION_REMOTE_DRIVE)
}

// WakeUp wakes up the vehicle.
func WakeUp() *vcsec.UnsignedMessage {
	return buildRKEAction(vcsec.RKEAction_E_RKE_ACTION_WAKE_VEHICLE)
}

// buildRKEAction builds an RKE action command to be sent to the vehicle.
// (RKE originally referred to "Remote Keyless Entry" but now refers more
// generally to commands that can be sent by a keyfob).
func buildRKEAction(action vcsec.RKEAction_E) *vcsec.UnsignedMessage {
	return &vcsec.UnsignedMessage{
		SubMessage: &vcsec.UnsignedMessage_RKEAction{
			RKEAction: action,
		},
	}
}
