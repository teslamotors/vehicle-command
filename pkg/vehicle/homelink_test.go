package vehicle

import (
	"context"
	"testing"
	"time"

	carserver "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/carserver"
	universal "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/universalmessage"
	"google.golang.org/protobuf/proto"
)

func extractHomelinkAction(t *testing.T, sender *testSender) *carserver.VehicleControlTriggerHomelinkAction {
	t.Helper()
	if sender.lastMessage == nil {
		t.Fatal("dispatcher did not capture a request")
	}
	var action carserver.Action
	if err := proto.Unmarshal(sender.lastMessage.GetProtobufMessageAsBytes(), &action); err != nil {
		t.Fatalf("unmarshal Action: %v", err)
	}
	hl := action.GetVehicleAction().GetVehicleControlTriggerHomelinkAction()
	if hl == nil {
		t.Fatalf("request was not VehicleControlTriggerHomelinkAction: %v", &action)
	}
	return hl
}

func newHomelinkTestVehicle(t *testing.T) (*Vehicle, *testSender) {
	t.Helper()
	car, sender := newTestVehicle()
	sender.Listen(nil)
	sender.fixedResponse = &universal.RoutableMessage{}
	return car, sender
}

func TestTriggerHomelinkOmitsDeviceSelector(t *testing.T) {
	car, sender := newHomelinkTestVehicle(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := car.TriggerHomelink(ctx, 37.5, -122.2); err != nil {
		t.Fatalf("TriggerHomelink: %v", err)
	}
	hl := extractHomelinkAction(t, sender)
	if got := hl.GetLocation().GetLatitude(); got != 37.5 {
		t.Errorf("latitude = %v, want 37.5", got)
	}
	if got := hl.GetLocation().GetLongitude(); got != -122.2 {
		t.Errorf("longitude = %v, want -122.2", got)
	}
	if hl.GetOptionalHomelinkDeviceIndex() != nil {
		t.Errorf("default TriggerHomelink encoded index %d; the field must be omitted so existing firmware keeps triggering the first device", hl.GetHomelinkDeviceIndex())
	}
	if hl.GetOptionalHomelinkDeviceName() != nil {
		t.Errorf("default TriggerHomelink encoded name %q; the field must be omitted", hl.GetHomelinkDeviceName())
	}
}

func TestTriggerHomelinkDeviceIndex(t *testing.T) {
	car, sender := newHomelinkTestVehicle(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := car.TriggerHomelinkDevice(ctx, 1, 2, HomelinkByIndex(2)); err != nil {
		t.Fatalf("TriggerHomelinkDevice: %v", err)
	}
	hl := extractHomelinkAction(t, sender)
	if hl.GetOptionalHomelinkDeviceIndex() == nil {
		t.Fatal("homelink_device_index was omitted")
	}
	if got := hl.GetHomelinkDeviceIndex(); got != 2 {
		t.Errorf("index = %d, want 2", got)
	}
	if hl.GetOptionalHomelinkDeviceName() != nil {
		t.Errorf("name %q was encoded alongside index; index should win", hl.GetHomelinkDeviceName())
	}
}

func TestTriggerHomelinkDeviceIndexZeroIsExplicit(t *testing.T) {
	car, sender := newHomelinkTestVehicle(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := car.TriggerHomelinkDevice(ctx, 1, 2, HomelinkByIndex(0)); err != nil {
		t.Fatalf("TriggerHomelinkDevice: %v", err)
	}
	hl := extractHomelinkAction(t, sender)
	if hl.GetOptionalHomelinkDeviceIndex() == nil {
		t.Fatal("index 0 must be encoded (oneof distinguishes it from 'unset')")
	}
	if got := hl.GetHomelinkDeviceIndex(); got != 0 {
		t.Errorf("index = %d, want 0", got)
	}
}

func TestTriggerHomelinkDeviceName(t *testing.T) {
	car, sender := newHomelinkTestVehicle(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := car.TriggerHomelinkDevice(ctx, 1, 2, HomelinkByName("  Garage Right  ")); err != nil {
		t.Fatalf("TriggerHomelinkDevice: %v", err)
	}
	hl := extractHomelinkAction(t, sender)
	if hl.GetOptionalHomelinkDeviceName() == nil {
		t.Fatal("homelink_device_name was omitted")
	}
	if got := hl.GetHomelinkDeviceName(); got != "Garage Right" {
		t.Errorf("name = %q, want %q", got, "Garage Right")
	}
	if hl.GetOptionalHomelinkDeviceIndex() != nil {
		t.Errorf("index %d was encoded alongside name", hl.GetHomelinkDeviceIndex())
	}
}

func TestTriggerHomelinkDeviceIndexTakesPrecedenceOverName(t *testing.T) {
	car, sender := newHomelinkTestVehicle(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	index := uint32(1)
	device := HomelinkDevice{Index: &index, Name: "Garage Right"}
	if err := car.TriggerHomelinkDevice(ctx, 1, 2, device); err != nil {
		t.Fatalf("TriggerHomelinkDevice: %v", err)
	}
	hl := extractHomelinkAction(t, sender)
	if hl.GetOptionalHomelinkDeviceIndex() == nil || hl.GetHomelinkDeviceIndex() != 1 {
		t.Errorf("index = %v, want 1", hl.GetOptionalHomelinkDeviceIndex())
	}
	if hl.GetOptionalHomelinkDeviceName() != nil {
		t.Errorf("name %q encoded when index was set", hl.GetHomelinkDeviceName())
	}
}

func TestHomelinkActionRoundTrip(t *testing.T) {
	original := &carserver.VehicleControlTriggerHomelinkAction{
		Location: &carserver.LatLong{Latitude: 37.5, Longitude: -122.2},
		OptionalHomelinkDeviceIndex: &carserver.VehicleControlTriggerHomelinkAction_HomelinkDeviceIndex{
			HomelinkDeviceIndex: 1,
		},
	}
	encoded, err := proto.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	var decoded carserver.VehicleControlTriggerHomelinkAction
	if err := proto.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.GetHomelinkDeviceIndex() != 1 {
		t.Errorf("round-trip index = %d, want 1", decoded.GetHomelinkDeviceIndex())
	}

	// Encoding without a selector must not grow vs. the two-field historical message.
	historical := &carserver.VehicleControlTriggerHomelinkAction{
		Location: &carserver.LatLong{Latitude: 37.5, Longitude: -122.2},
	}
	histBytes, err := proto.Marshal(historical)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) <= len(histBytes) {
		t.Errorf("selector encoding did not add bytes (got %d, historical %d)", len(encoded), len(histBytes))
	}
}
