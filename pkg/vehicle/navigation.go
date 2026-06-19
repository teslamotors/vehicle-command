package vehicle

import (
	"context"

	carserver "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/carserver"
)

// NavigateToWaypoints sends an ordered list of stops to the vehicle's
// built-in navigation using the signed Vehicle Command Protocol.
//
// The `waypoints` argument follows Tesla's documented format:
// a comma-separated string of `refId:<Google Maps place ID>` tokens, e.g.
//
//	"refId:ChIJxxx,refId:ChIJyyy,refId:ChIJzzz"
//
// The last place in the list is the final destination; preceding entries are
// intermediate stops. The vehicle will route through all of them in order.
//
// See https://developer.tesla.com/docs/fleet-api/endpoints/vehicle-commands#navigation-waypoints-request
func (v *Vehicle) NavigateToWaypoints(ctx context.Context, waypoints string) error {
	return v.NavigateToWaypointsWithOptions(ctx, waypoints, 0, 0)
}

// NavigateToWaypointsWithOptions is like NavigateToWaypoints but allows the
// caller to set the optional `TripPlanOptions.destination_start_soe` and
// `TripPlanOptions.destination_arrival_soe` fields (both in integer
// percent). Pass 0 to leave a field unset (Tesla will use its own default).
func (v *Vehicle) NavigateToWaypointsWithOptions(ctx context.Context, waypoints string, startSoePct, arrivalSoePct int32) error {
	req := &carserver.NavigationWaypointsRequest{
		Waypoints: waypoints,
	}
	if startSoePct != 0 || arrivalSoePct != 0 {
		req.TripPlanOptions = &carserver.NavigationWaypointsRequest_TripPlanOptions{
			DestinationStartSoe:   startSoePct,
			DestinationArrivalSoe: arrivalSoePct,
		}
	}
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_NavigationWaypointsRequest{
					NavigationWaypointsRequest: req,
				},
			},
		})
}
