package vehicle

import (
	"context"

	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/carserver"
)

func (v *Vehicle) MediaNextTrack(ctx context.Context) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_MediaNextTrack{},
			},
		})
}

func (v *Vehicle) MediaPreviousTrack(ctx context.Context) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_MediaPreviousTrack{},
			},
		})
}

func (v *Vehicle) MediaNextFavorite(ctx context.Context) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_MediaNextFavorite{},
			},
		})
}

func (v *Vehicle) MediaPreviousFavorite(ctx context.Context) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_MediaPreviousFavorite{},
			},
		})
}

func (v *Vehicle) MediaVolumeRelative(ctx context.Context, delta int32) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_MediaUpdateVolume{
					MediaUpdateVolume: &carserver.MediaUpdateVolume{
						MediaVolume: &carserver.MediaUpdateVolume_VolumeDelta{
							VolumeDelta: int32(delta),
						},
					},
				},
			},
		})
}
