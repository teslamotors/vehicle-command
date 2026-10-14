package vehicle

import (
	"context"
	"testing"
	"time"

	carserver "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/carserver"
	universal "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/universalmessage"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

func extractCarServerAction(t *testing.T, sender *testSender) *carserver.Action {
	t.Helper()
	if sender.lastMessage == nil {
		t.Fatal("dispatcher did not capture a request")
	}
	var action carserver.Action
	if err := proto.Unmarshal(sender.lastMessage.GetProtobufMessageAsBytes(), &action); err != nil {
		t.Fatalf("unmarshal Action: %v", err)
	}
	return &action
}

func newSuspensionTestVehicle(t *testing.T) (*Vehicle, *testSender) {
	t.Helper()
	car, sender := newTestVehicle()
	sender.Listen(nil)
	sender.fixedResponse = &universal.RoutableMessage{}
	return car, sender
}

func TestParseSuspensionLevel(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in      string
		want    SuspensionLevel
		wantErr bool
	}{
		{in: "entry", want: SuspensionLevelEntry},
		{in: "EASY_ENTRY", want: SuspensionLevelEntry},
		{in: "low", want: SuspensionLevelLow},
		{in: "medium", want: SuspensionLevelMedium},
		{in: "standard", want: SuspensionLevelMedium},
		{in: "level", want: SuspensionLevelMedium},
		{in: "high", want: SuspensionLevelHigh},
		{in: "very-high", want: SuspensionLevelVeryHigh},
		{in: "extract", want: SuspensionLevelExtract},
		{in: "3", want: SuspensionLevelMedium},
		{in: "6", want: SuspensionLevelExtract},
		{in: "0", wantErr: true},
		{in: "invalid", wantErr: true},
		{in: "", wantErr: true},
	}
	for _, tc := range cases {
		got, err := ParseSuspensionLevel(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseSuspensionLevel(%q) = %v, want error", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseSuspensionLevel(%q): %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseSuspensionLevel(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestSetTentModeEncodesField94(t *testing.T) {
	car, sender := newSuspensionTestVehicle(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := car.SetTentMode(ctx, true); err != nil {
		t.Fatalf("SetTentMode: %v", err)
	}
	action := extractCarServerAction(t, sender)
	tent := action.GetVehicleAction().GetSetTentModeRequestAction()
	if tent == nil {
		t.Fatalf("request was not SetTentModeRequestAction: %v", action)
	}
	if !tent.GetOn() {
		t.Error("on = false, want true")
	}

	if got := vehicleActionFieldNumber(t, action); got != 94 {
		t.Errorf("VehicleAction oneof field = %d, want 94 (setTentModeRequestAction)", got)
	}
}

func TestSetTentModeOff(t *testing.T) {
	car, sender := newSuspensionTestVehicle(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := car.SetTentMode(ctx, false); err != nil {
		t.Fatalf("SetTentMode: %v", err)
	}
	tent := extractCarServerAction(t, sender).GetVehicleAction().GetSetTentModeRequestAction()
	if tent == nil || tent.GetOn() {
		t.Errorf("tent = %v, want on=false", tent)
	}
}

func TestSetSuspensionLevelEncodesField118(t *testing.T) {
	car, sender := newSuspensionTestVehicle(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := car.SetSuspensionLevel(ctx, SuspensionLevelMedium); err != nil {
		t.Fatalf("SetSuspensionLevel: %v", err)
	}
	action := extractCarServerAction(t, sender)
	susp := action.GetVehicleAction().GetSetSuspensionLevelAction()
	if susp == nil {
		t.Fatalf("request was not SetSuspensionLevelAction: %v", action)
	}
	if susp.GetSuspensionLevel() != carserver.SetSuspensionLevelAction_SUSPENSION_LEVEL_MEDIUM {
		t.Errorf("level = %v, want MEDIUM", susp.GetSuspensionLevel())
	}

	if got := vehicleActionFieldNumber(t, action); got != 118 {
		t.Errorf("VehicleAction oneof field = %d, want 118 (setSuspensionLevelAction)", got)
	}
}

func TestLevelSuspensionUsesMedium(t *testing.T) {
	car, sender := newSuspensionTestVehicle(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := car.LevelSuspension(ctx); err != nil {
		t.Fatalf("LevelSuspension: %v", err)
	}
	susp := extractCarServerAction(t, sender).GetVehicleAction().GetSetSuspensionLevelAction()
	if susp == nil || susp.GetSuspensionLevel() != carserver.SetSuspensionLevelAction_SUSPENSION_LEVEL_MEDIUM {
		t.Errorf("LevelSuspension encoded %v, want MEDIUM", susp)
	}
}

func vehicleActionFieldNumber(t *testing.T, action *carserver.Action) protowire.Number {
	t.Helper()
	encoded, err := proto.Marshal(action.GetVehicleAction())
	if err != nil {
		t.Fatal(err)
	}
	num, _, n := protowire.ConsumeTag(encoded)
	if n < 0 {
		t.Fatalf("VehicleAction wire tag: %v", n)
	}
	return num
}
