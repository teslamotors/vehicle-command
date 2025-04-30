package proxy

import (
    "context"
    "log"
    "github.com/teslamotors/vehicle-command/pkg/protocol"
    "github.com/teslamotors/vehicle-command/pkg/vehicle"
    "github.com/Sensei-Intent-Tensor/intent-tensor-vehicle-module/intent"
)

// deriveUrgency computes a default urgency level based on speed and recent actions
func deriveUrgency(v *vehicle.Vehicle) float64 {
    speed := v.LastKnownSpeed()
    if speed < 5.0 {
        return 0.2 // calm
    } else if speed < 30.0 {
        return 0.5 // moderate
    }
    return 0.9 // high urgency
}

// WrapWithIntentCheck applies intent filtering before executing a command
func WrapWithIntentCheck(cmdName string, handler func(context.Context, *vehicle.Vehicle) error) func(context.Context, *vehicle.Vehicle) error {
    return func(ctx context.Context, v *vehicle.Vehicle) error {
        signal := intent.TelemetrySignal{
            Speed:       v.LastKnownSpeed(),
            Obstacle:    false,              // Not exposed, assume false by default
            Urgency:     deriveUrgency(v),
            Temperature: v.CabinTemp(),      // Replace with external temp if needed
        }

        state := intent.CollapseIntent(signal)

        if !intent.ShouldAllowCommand(state, cmdName) {
            log.Printf("[INTENT BLOCKED] Command '%s' was blocked (risk: %.2f)", cmdName, state.RiskProfile)
            return protocol.NewError("intent_blocked", false, false)
        }

        return handler(ctx, v)
    }
}

# Add intent-aware decision wrapper (imported from region-aware-routing)
