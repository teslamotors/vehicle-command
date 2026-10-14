package vehicle

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	carserver "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/carserver"
)

// SuspensionLevel is an air-suspension ride height. MEDIUM is the standard
// height; ENTRY is the lowest (easy entry / cargo loading).
type SuspensionLevel = carserver.SetSuspensionLevelAction_SuspensionLevel

const (
	SuspensionLevelInvalid  = carserver.SetSuspensionLevelAction_SUSPENSION_LEVEL_INVALID
	SuspensionLevelEntry    = carserver.SetSuspensionLevelAction_SUSPENSION_LEVEL_ENTRY
	SuspensionLevelLow      = carserver.SetSuspensionLevelAction_SUSPENSION_LEVEL_LOW
	SuspensionLevelMedium   = carserver.SetSuspensionLevelAction_SUSPENSION_LEVEL_MEDIUM
	SuspensionLevelHigh     = carserver.SetSuspensionLevelAction_SUSPENSION_LEVEL_HIGH
	SuspensionLevelVeryHigh = carserver.SetSuspensionLevelAction_SUSPENSION_LEVEL_VERY_HIGH
	SuspensionLevelExtract  = carserver.SetSuspensionLevelAction_SUSPENSION_LEVEL_EXTRACT
)

// ParseSuspensionLevel maps CLI / JSON names (and 1–6) onto a ride height.
// "level" is accepted as an alias for medium, which is the height tent mode
// uses when auto-leveling is not available. INVALID is rejected.
func ParseSuspensionLevel(s string) (SuspensionLevel, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "entry", "easy-entry", "easy_entry":
		return SuspensionLevelEntry, nil
	case "low":
		return SuspensionLevelLow, nil
	case "medium", "standard", "level":
		return SuspensionLevelMedium, nil
	case "high":
		return SuspensionLevelHigh, nil
	case "very-high", "very_high", "veryhigh":
		return SuspensionLevelVeryHigh, nil
	case "extract":
		return SuspensionLevelExtract, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 || n > 6 {
		return SuspensionLevelInvalid, fmt.Errorf("suspension level must be entry, low, medium, high, very-high, extract, or 1-6")
	}
	return SuspensionLevel(n), nil
}

// SetTentMode enables or disables Cybertruck tent mode. The vehicle typically
// refuses unless it is in Park and the hardware supports tent mode. Tent mode
// also auto-levels the air suspension; use SetSuspensionLevel to pick a ride
// height without entering tent mode. See teslamotors/vehicle-command#424.
func (v *Vehicle) SetTentMode(ctx context.Context, on bool) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_SetTentModeRequestAction{
					SetTentModeRequestAction: &carserver.SetTentModeRequestAction{
						On: on,
					},
				},
			},
		})
}

// SetSuspensionLevel sets air-suspension ride height. Vehicles typically
// refuse unless they are in Park and have adaptive air suspension. Use
// SetTentMode when the goal is Cybertruck tent-mode auto-leveling rather than
// a preset height.
func (v *Vehicle) SetSuspensionLevel(ctx context.Context, level SuspensionLevel) error {
	return v.executeCarServerAction(ctx,
		&carserver.Action_VehicleAction{
			VehicleAction: &carserver.VehicleAction{
				VehicleActionMsg: &carserver.VehicleAction_SetSuspensionLevelAction{
					SetSuspensionLevelAction: &carserver.SetSuspensionLevelAction{
						SuspensionLevel: level,
					},
				},
			},
		})
}

// LevelSuspension sets ride height to MEDIUM (standard). Prefer SetTentMode
// when the vehicle should auto-level each corner for a cargo bed or tent.
func (v *Vehicle) LevelSuspension(ctx context.Context) error {
	return v.SetSuspensionLevel(ctx, SuspensionLevelMedium)
}
