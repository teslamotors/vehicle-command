package vehicle

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/teslamotors/vehicle-command/pkg/protocol"
	carserver "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/carserver"
	universal "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/universalmessage"
	"google.golang.org/protobuf/proto"
)

func TestScheduleChargingDeliversPublishedAction(t *testing.T) {
	car, sender := newTestVehicle()
	sender.Listen(nil)
	sender.fixedResponse = &universal.RoutableMessage{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := car.ScheduleCharging(ctx, true, 2*time.Hour); err != nil {
		t.Fatalf("ScheduleCharging: %v", err)
	}
	if sender.lastMessage == nil {
		t.Fatal("dispatcher did not capture a request")
	}
	var action carserver.Action
	if err := proto.Unmarshal(sender.lastMessage.GetProtobufMessageAsBytes(), &action); err != nil {
		t.Fatalf("unmarshal Action: %v", err)
	}
	got := action.GetVehicleAction().GetScheduledChargingAction()
	if got == nil {
		t.Fatalf("request was not ScheduledChargingAction: %v", &action)
	}
	if !got.GetEnabled() {
		t.Error("enabled = false, want true")
	}
	if got.GetChargingTime() != 120 {
		t.Errorf("charging_time = %d, want 120 minutes", got.GetChargingTime())
	}
}

func TestScheduleChargingDoesNotReturnFirmwareWorkaroundError(t *testing.T) {
	car, sender := newTestVehicle()
	sender.Listen(nil)
	sender.fixedResponse = &universal.RoutableMessage{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := car.ScheduleCharging(ctx, true, time.Hour)
	if errors.Is(err, protocol.ErrScheduledChargingFirmware) {
		t.Fatal("ScheduleCharging must still send the published action; firmware sleep/wake is not a client-side refusal")
	}
	if err != nil {
		t.Fatalf("ScheduleCharging: %v", err)
	}
}

func TestOpenChargePortDeliversPublishedAction(t *testing.T) {
	car, sender := newTestVehicle()
	sender.Listen(nil)
	sender.fixedResponse = &universal.RoutableMessage{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := car.OpenChargePort(ctx); err != nil {
		if errors.Is(err, protocol.ErrChargingManagerChargePortFirmware) {
			t.Fatal("OpenChargePort must still send ChargePortDoorOpen; Charging Manager ACL is firmware")
		}
		t.Fatalf("OpenChargePort: %v", err)
	}
	if sender.lastMessage == nil {
		t.Fatal("dispatcher did not capture a request")
	}
	var action carserver.Action
	if err := proto.Unmarshal(sender.lastMessage.GetProtobufMessageAsBytes(), &action); err != nil {
		t.Fatalf("unmarshal Action: %v", err)
	}
	if action.GetVehicleAction().GetChargePortDoorOpen() == nil {
		t.Fatalf("request was not ChargePortDoorOpen: %v", &action)
	}
}

func TestCloseChargePortDeliversPublishedAction(t *testing.T) {
	car, sender := newTestVehicle()
	sender.Listen(nil)
	sender.fixedResponse = &universal.RoutableMessage{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := car.CloseChargePort(ctx); err != nil {
		if errors.Is(err, protocol.ErrChargingManagerChargePortFirmware) {
			t.Fatal("CloseChargePort must still send ChargePortDoorClose")
		}
		t.Fatalf("CloseChargePort: %v", err)
	}
	var action carserver.Action
	if err := proto.Unmarshal(sender.lastMessage.GetProtobufMessageAsBytes(), &action); err != nil {
		t.Fatalf("unmarshal Action: %v", err)
	}
	if action.GetVehicleAction().GetChargePortDoorClose() == nil {
		t.Fatalf("request was not ChargePortDoorClose: %v", &action)
	}
}

func TestChargeStopDeliversPublishedAction(t *testing.T) {
	car, sender := newTestVehicle()
	sender.Listen(nil)
	sender.fixedResponse = &universal.RoutableMessage{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := car.ChargeStop(ctx); err != nil {
		if errors.Is(err, protocol.ErrChargingWhileInfotainmentAsleep) {
			t.Fatal("ChargeStop must still send ChargingStartStopAction; Infotainment sleep is Tesla signed_command, not a client-side refusal")
		}
		t.Fatalf("ChargeStop: %v", err)
	}
	if sender.lastMessage == nil {
		t.Fatal("dispatcher did not capture a request")
	}
	var action carserver.Action
	if err := proto.Unmarshal(sender.lastMessage.GetProtobufMessageAsBytes(), &action); err != nil {
		t.Fatalf("unmarshal Action: %v", err)
	}
	stop := action.GetVehicleAction().GetChargingStartStopAction()
	if stop == nil || stop.GetStop() == nil {
		t.Fatalf("request was not ChargingStartStopAction stop: %v", &action)
	}
}

func TestSetChargingAmpsDeliversPublishedAction(t *testing.T) {
	car, sender := newTestVehicle()
	sender.Listen(nil)
	sender.fixedResponse = &universal.RoutableMessage{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := car.SetChargingAmps(ctx, 16); err != nil {
		if errors.Is(err, protocol.ErrChargingWhileInfotainmentAsleep) {
			t.Fatal("SetChargingAmps must still send SetChargingAmpsAction; Infotainment sleep is Tesla signed_command, not a client-side refusal")
		}
		t.Fatalf("SetChargingAmps: %v", err)
	}
	if sender.lastMessage == nil {
		t.Fatal("dispatcher did not capture a request")
	}
	var action carserver.Action
	if err := proto.Unmarshal(sender.lastMessage.GetProtobufMessageAsBytes(), &action); err != nil {
		t.Fatalf("unmarshal Action: %v", err)
	}
	got := action.GetVehicleAction().GetSetChargingAmpsAction()
	if got == nil {
		t.Fatalf("request was not SetChargingAmpsAction: %v", &action)
	}
	if got.GetChargingAmps() != 16 {
		t.Errorf("charging_amps = %d, want 16", got.GetChargingAmps())
	}
}

func TestSetChargingAmpsSendsRequestedValueBelowFive(t *testing.T) {
	for _, amps := range []int32{0, 1, 2, 5} {
		car, sender := newTestVehicle()
		sender.Listen(nil)
		sender.fixedResponse = &universal.RoutableMessage{}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		err := car.SetChargingAmps(ctx, amps)
		cancel()
		if errors.Is(err, protocol.ErrChargingAmpsBelowFloorFirmware) {
			t.Fatalf("SetChargingAmps(%d) must still send the requested integer", amps)
		}
		if err != nil {
			t.Fatalf("SetChargingAmps(%d): %v", amps, err)
		}
		var action carserver.Action
		if err := proto.Unmarshal(sender.lastMessage.GetProtobufMessageAsBytes(), &action); err != nil {
			t.Fatalf("unmarshal Action: %v", err)
		}
		got := action.GetVehicleAction().GetSetChargingAmpsAction()
		if got == nil || got.GetChargingAmps() != amps {
			t.Fatalf("charging_amps = %v, want %d", got, amps)
		}
	}
}
