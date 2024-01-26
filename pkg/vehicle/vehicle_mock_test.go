package vehicle_test

import "context"

type MockVehicle struct{}

func (v *MockVehicle) SetVolume(ctx context.Context, volume float32) error {
	return nil
}
