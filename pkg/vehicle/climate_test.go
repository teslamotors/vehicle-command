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

func climateActionFromLastMessage(t *testing.T, sender *testSender) *carserver.VehicleAction {
	t.Helper()
	if sender.lastMessage == nil {
		t.Fatal("dispatcher did not capture a request")
	}
	var action carserver.Action
	if err := proto.Unmarshal(sender.lastMessage.GetProtobufMessageAsBytes(), &action); err != nil {
		t.Fatalf("unmarshal Action: %v", err)
	}
	got := action.GetVehicleAction()
	if got == nil {
		t.Fatalf("request was not VehicleAction: %v", &action)
	}
	return got
}

func TestClimateOnSendsHvacAutoActionPower(t *testing.T) {
	car, sender := newTestVehicle()
	sender.Listen(nil)
	sender.fixedResponse = &universal.RoutableMessage{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := car.ClimateOn(ctx); err != nil {
		if errors.Is(err, protocol.ErrHvacAutoModeNotInProtocol) {
			t.Fatal("ClimateOn must send published HvacAutoAction.power_on, not refuse Auto vs Manual")
		}
		t.Fatalf("ClimateOn: %v", err)
	}
	hvac := climateActionFromLastMessage(t, sender).GetHvacAutoAction()
	if hvac == nil {
		t.Fatal("request was not HvacAutoAction")
	}
	if !hvac.GetPowerOn() {
		t.Error("power_on = false, want true")
	}
	if hvac.GetManualOverride() {
		t.Error("ClimateOn must leave manual_override false")
	}
}

func TestSetClimatePowerManualOverride(t *testing.T) {
	car, sender := newTestVehicle()
	sender.Listen(nil)
	sender.fixedResponse = &universal.RoutableMessage{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := car.SetClimatePower(ctx, true, true); err != nil {
		t.Fatalf("SetClimatePower: %v", err)
	}
	hvac := climateActionFromLastMessage(t, sender).GetHvacAutoAction()
	if hvac == nil {
		t.Fatal("request was not HvacAutoAction")
	}
	if !hvac.GetPowerOn() || !hvac.GetManualOverride() {
		t.Errorf("power_on=%v manual_override=%v, want true/true", hvac.GetPowerOn(), hvac.GetManualOverride())
	}
}

func TestClimateOffSendsHvacAutoActionPowerOff(t *testing.T) {
	car, sender := newTestVehicle()
	sender.Listen(nil)
	sender.fixedResponse = &universal.RoutableMessage{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := car.ClimateOff(ctx); err != nil {
		t.Fatalf("ClimateOff: %v", err)
	}
	hvac := climateActionFromLastMessage(t, sender).GetHvacAutoAction()
	if hvac == nil {
		t.Fatal("request was not HvacAutoAction")
	}
	if hvac.GetPowerOn() {
		t.Error("power_on = true, want false")
	}
}

func TestChangeClimateTempEncodesDriverAndPassenger(t *testing.T) {
	car, sender := newTestVehicle()
	sender.Listen(nil)
	sender.fixedResponse = &universal.RoutableMessage{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := car.ChangeClimateTemp(ctx, 22, 21); err != nil {
		if errors.Is(err, protocol.ErrClimateSplitNotInProtocol) {
			t.Fatal("ChangeClimateTemp must send published HvacTemperatureAdjustmentAction, not refuse climate split")
		}
		t.Fatalf("ChangeClimateTemp: %v", err)
	}
	adj := climateActionFromLastMessage(t, sender).GetHvacTemperatureAdjustmentAction()
	if adj == nil {
		t.Fatal("request was not HvacTemperatureAdjustmentAction")
	}
	if adj.GetDriverTempCelsius() != 22 {
		t.Errorf("driver_temp_celsius = %v, want 22", adj.GetDriverTempCelsius())
	}
	if adj.GetPassengerTempCelsius() != 21 {
		t.Errorf("passenger_temp_celsius = %v, want 21", adj.GetPassengerTempCelsius())
	}
	if adj.GetAbsoluteCelsius() != 0 {
		t.Errorf("absolute_celsius = %v, want unset (0); firmware ignores it when driver_temp is the setpoint", adj.GetAbsoluteCelsius())
	}
	if adj.GetLevel() != nil {
		t.Errorf("level = %v, want unset (TEMP_MIN/TEMP_MAX are LO/HI, not a numeric-temp flag)", adj.GetLevel())
	}
}

func TestSetSeatHeaterSendsHvacSeatHeaterActions(t *testing.T) {
	car, sender := newTestVehicle()
	sender.Listen(nil)
	sender.fixedResponse = &universal.RoutableMessage{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := car.SetSeatHeater(ctx, map[SeatPosition]Level{SeatFrontLeft: LevelMed}); err != nil {
		if errors.Is(err, protocol.ErrSeatClimateFleetAPI) {
			t.Fatal("SetSeatHeater must send published HvacSeatHeaterActions, not refuse as unimplemented")
		}
		t.Fatalf("SetSeatHeater: %v", err)
	}
	heater := climateActionFromLastMessage(t, sender).GetHvacSeatHeaterActions()
	if heater == nil {
		t.Fatal("request was not HvacSeatHeaterActions")
	}
	if len(heater.HvacSeatHeaterAction) != 1 {
		t.Fatalf("actions = %d, want 1", len(heater.HvacSeatHeaterAction))
	}
	action := heater.HvacSeatHeaterAction[0]
	if action.GetSEAT_HEATER_MED() == nil {
		t.Error("seat heater level is not MED")
	}
	if action.GetCAR_SEAT_FRONT_LEFT() == nil {
		t.Error("seat position is not FRONT_LEFT")
	}
}

func TestSetSeatCoolerSendsHvacSeatCoolerActions(t *testing.T) {
	car, sender := newTestVehicle()
	sender.Listen(nil)
	sender.fixedResponse = &universal.RoutableMessage{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := car.SetSeatCooler(ctx, LevelLow, SeatFrontRight); err != nil {
		if errors.Is(err, protocol.ErrSeatClimateFleetAPI) {
			t.Fatal("SetSeatCooler must send published HvacSeatCoolerActions, not refuse as unimplemented")
		}
		t.Fatalf("SetSeatCooler: %v", err)
	}
	cooler := climateActionFromLastMessage(t, sender).GetHvacSeatCoolerActions()
	if cooler == nil {
		t.Fatal("request was not HvacSeatCoolerActions")
	}
	if len(cooler.HvacSeatCoolerAction) != 1 {
		t.Fatalf("actions = %d, want 1", len(cooler.HvacSeatCoolerAction))
	}
	action := cooler.HvacSeatCoolerAction[0]
	if action.GetSeatCoolerLevel() != carserver.HvacSeatCoolerActions_HvacSeatCoolerLevel_Low {
		t.Errorf("seat_cooler_level = %v, want Low", action.GetSeatCoolerLevel())
	}
	if action.GetSeatPosition() != carserver.HvacSeatCoolerActions_HvacSeatCoolerPosition_FrontRight {
		t.Errorf("seat_position = %v, want FrontRight", action.GetSeatPosition())
	}
}

func TestSetClimateKeeperModeDeliversPublishedAction(t *testing.T) {
	car, sender := newTestVehicle()
	sender.Listen(nil)
	sender.fixedResponse = &universal.RoutableMessage{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := car.SetClimateKeeperMode(ctx, ClimateKeeperModeDog, false); err != nil {
		if errors.Is(err, protocol.ErrClimateKeeperCPDFirmware) {
			t.Fatal("SetClimateKeeperMode must still send HvacClimateKeeperAction; cpd_enabled is firmware NominalError, not a client-side refusal")
		}
		t.Fatalf("SetClimateKeeperMode: %v", err)
	}
	got := climateActionFromLastMessage(t, sender).GetHvacClimateKeeperAction()
	if got == nil {
		t.Fatal("request was not HvacClimateKeeperAction")
	}
	if got.GetClimateKeeperAction() != ClimateKeeperModeDog {
		t.Errorf("mode = %v, want Dog", got.GetClimateKeeperAction())
	}
	if got.GetManualOverride() {
		t.Error("manual_override = true, want false")
	}
}

func TestSetClimateKeeperModeManualOverrideIsNotCPDBypass(t *testing.T) {
	car, sender := newTestVehicle()
	sender.Listen(nil)
	sender.fixedResponse = &universal.RoutableMessage{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := car.SetClimateKeeperMode(ctx, ClimateKeeperModeCamp, true); err != nil {
		if errors.Is(err, protocol.ErrClimateKeeperCPDFirmware) {
			t.Fatal("manual_override must still be sent as the published low-SOC bit; firmware may still refuse with cpd_enabled")
		}
		t.Fatalf("SetClimateKeeperMode: %v", err)
	}
	got := climateActionFromLastMessage(t, sender).GetHvacClimateKeeperAction()
	if got == nil {
		t.Fatal("request was not HvacClimateKeeperAction")
	}
	if got.GetClimateKeeperAction() != ClimateKeeperModeCamp {
		t.Errorf("mode = %v, want Camp", got.GetClimateKeeperAction())
	}
	if !got.GetManualOverride() {
		t.Error("manual_override = false, want true")
	}
}
