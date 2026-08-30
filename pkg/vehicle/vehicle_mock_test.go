package vehicle_test

import "context"

type MockVehicle struct{}

func (v *MockVehicle) SetVolume(_ context.Context, _ float32) error {
	return nil
}
