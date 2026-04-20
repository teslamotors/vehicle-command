package vehicle

import (
	"context"

	carserver "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/carserver"
)

// NavigateToWaypoints sends an ordered list of stops to the car's
// built-in navigation using the signed Vehicle Command Protocol.
//
// The `waypoints` argument follows Tesla's documented format:
// a comma-separated string of `refId:<Google Maps place ID>` tokens, e.g.
//
//	"refId:ChIJxxx,refId:ChIJyyy,refId:ChIJzzz"
//
// The last place in the list is the final destination; preceding entries are
// intermediate stops. The car will route through all of them in order.
//
// See https://developer.tesla.com/docs/fleet-api/endpoints/vehicle-commands
// (navigation_waypoints_request).
func (v *Vehicle) NavigateToWaypoints(ctx context.Context, waypoints string) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_NavigationWaypointsRequest{
					NavigationWaypointsRequest: &carserver.NavigationWaypointsRequest{
						Waypoints: waypoints,
					},
				},
			},
		})
}
