package vehicle

import (
	"context"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/vcsec"
)

// BodyControllerState returns information about closures, locks, and infotainment sleep status.
// This method works over BLE even when infotainment is asleep.
func (v *Vehicle) BodyControllerState(ctx context.Context) (*vcsec.VehicleStatus, error) {
	reply, err := v.getVCSECInfo(ctx, vcsec.InformationRequestType_INFORMATION_REQUEST_TYPE_GET_STATUS, slotNone)
	if err != nil {
		return nil, err
	}
	return reply.GetVehicleStatus(), nil
}
