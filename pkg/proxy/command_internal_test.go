package proxy

import (
	"testing"

	"github.com/teslamotors/vehicle-command/pkg/vehicle"
)

func TestSettingForCoolerSeatPosition(t *testing.T) {
	for _, tc := range []struct {
		level float64
		want  vehicle.Level
	}{
		{0, vehicle.LevelOff},
		{1, vehicle.LevelLow},
		{2, vehicle.LevelMed},
		{3, vehicle.LevelHigh},
	} {
		params := RequestParameters{"seat_position": float64(1), "seat_cooler_level": tc.level}
		level, seat, err := params.settingForCoolerSeatPosition()
		if err != nil {
			t.Fatalf("seat_cooler_level %v: %s", tc.level, err)
		}
		if level != tc.want {
			t.Errorf("seat_cooler_level %v: got %v, want %v", tc.level, level, tc.want)
		}
		if seat == vehicle.SeatUnknown {
			t.Errorf("seat_position 1: got SeatUnknown")
		}
	}
}
