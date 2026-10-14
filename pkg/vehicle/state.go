package vehicle

import (
	"context"
	"fmt"

	carserver "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/carserver"
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

type StateCategory int32

const (
	StateCategoryCharge StateCategory = iota
	// StateCategoryClimate returns [carserver.ClimateState], including
	// driver_temp_setting and passenger_temp_setting. Tesla has not published
	// a climate split/SYNC boolean; teslamotors/vehicle-command#386.
	StateCategoryClimate
	// StateCategoryDrive returns [carserver.DriveState], which carries gear, speed, power, and
	// odometer as well as the active navigation route. See the note on response size limits in
	// [Vehicle.GetState].
	StateCategoryDrive
	StateCategoryLocation
	StateCategoryClosures
	StateCategoryChargeSchedule
	StateCategoryPreconditioningSchedule
	StateCategoryTirePressure
	StateCategoryMedia
	StateCategoryMediaDetail
	StateCategorySoftwareUpdate
	StateCategoryParentalControls
)

func (c StateCategory) submessage() *carserver.GetVehicleData {
	messages := map[StateCategory]*carserver.GetVehicleData{
		StateCategoryCharge:                  {GetChargeState: &carserver.GetChargeState{}},
		StateCategoryClimate:                 {GetClimateState: &carserver.GetClimateState{}},
		StateCategoryDrive:                   {GetDriveState: &carserver.GetDriveState{}},
		StateCategoryLocation:                {GetLocationState: &carserver.GetLocationState{}},
		StateCategoryClosures:                {GetClosuresState: &carserver.GetClosuresState{}},
		StateCategoryChargeSchedule:          {GetChargeScheduleState: &carserver.GetChargeScheduleState{}},
		StateCategoryPreconditioningSchedule: {GetPreconditioningScheduleState: &carserver.GetPreconditioningScheduleState{}},
		StateCategoryTirePressure:            {GetTirePressureState: &carserver.GetTirePressureState{}},
		StateCategoryMedia:                   {GetMediaState: &carserver.GetMediaState{}},
		StateCategoryMediaDetail:             {GetMediaDetailState: &carserver.GetMediaDetailState{}},
		StateCategorySoftwareUpdate:          {GetSoftwareUpdateState: &carserver.GetSoftwareUpdateState{}},
		StateCategoryParentalControls:        {GetParentalControlsState: &carserver.GetParentalControlsState{}},
	}
	msg, ok := messages[c]
	if !ok {
		return nil
	}
	return msg
}

// GetState fetches vehicle information.
//
// This is intended for use over BLE. The [vehicle data] Fleet API endpoint is much more efficient
// for clients that connect over the Internet because it combines data into a single query and can
// serve cached data when the vehicle is offline.
//
// StateCategoryLocation may return a few different (latitude, longitude) fields. See
// [carserver.LocationState] documentation for an explanation.
//
// # BLE poll latency
//
// Each GetState is one signed Infotainment round-trip. Callers have measured
// ~250–300ms of BLE time versus a few milliseconds of client construction and
// encryption (teslamotors/vehicle-command#414). That floor is vehicle
// firmware plus radio, not a client sleep. Handshake once (StartSession) and
// reuse the session; reconnecting or requesting extra categories adds another
// RTT (~500ms for handshake+GetState). There is no published streaming
// DriveState VehicleAction. Do not disable FLAG_ENCRYPT_RESPONSE or shorten
// UUIDs to chase a sub-150ms poll. High-rate streaming is
// teslamotors/fleet-telemetry. Clients that ask this library to guarantee
// <150ms get protocol.ErrBLEStateLatencyFirmware from tesla-control
// ble-state-fast rather than an unsafe workaround.
//
// # Response size limits
//
// The vehicle enforces a fixed ceiling on the size of a serialized response. If the requested
// state does not fit, the vehicle discards the reply and returns a [protocol.RoutableMessageError]
// with Code MESSAGEFAULT_ERROR_RESPONSE_MTU_EXCEEDED instead of a partial payload. The ceiling is
// a vehicle-side memory budget, not the negotiated BLE MTU, so reconnecting does not help, and the
// request messages carry no field mask that would let a client ask for a smaller reply.
//
// In practice this affects StateCategoryDrive: the vehicle always populates the
// DriveState.active_route_* fields while navigation is active, and a long destination name can
// push the response over the limit. The failure clears on its own once the route ends or the
// destination changes. Clients that poll DriveState should treat this error as "no new sample"
// and keep their last known values rather than retrying in a tight loop; GetState already does
// not retry it because retransmitting the same request produces the same oversized reply. See
// https://github.com/teslamotors/vehicle-command/issues/472 for measurements.
//
// [vehicle data]: https://developer.tesla.com/docs/fleet-api/endpoints/vehicle-endpoints#vehicle-data
func (v *Vehicle) GetState(ctx context.Context, category StateCategory) (*carserver.VehicleData, error) {
	submessage := category.submessage()
	if submessage == nil {
		return nil, fmt.Errorf("unrecognized vehicle data category")
	}
	action := carserver.Action_VehicleAction{
		VehicleAction: &carserver.VehicleAction{
			VehicleActionMsg: &carserver.VehicleAction_GetVehicleData{
				GetVehicleData: submessage,
			},
		},
	}
	rsp, err := v.getCarServerResponse(ctx, &action)
	if err != nil {
		return nil, err
	}
	return rsp.GetVehicleData(), nil
}
