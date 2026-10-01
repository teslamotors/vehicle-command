package main

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/teslamotors/vehicle-command/pkg/protocol"
	"github.com/teslamotors/vehicle-command/pkg/vehicle"
)

func TestMinutesAfterMidnight(t *testing.T) {
	type params struct {
		str     string
		minutes int32
		err     error
	}
	testCases := []params{
		{str: "3:03", minutes: 183},
		{str: "0:00", minutes: 0},
		{str: "", err: ErrInvalidTime},
		{str: "3:", err: ErrInvalidTime},
		{str: ":40", err: ErrInvalidTime},
		{str: "3:40pm", err: ErrInvalidTime},
		{str: "25:40", err: ErrInvalidTime},
		{str: "23:40", minutes: 23*60 + 40},
		{str: "23:60", err: ErrInvalidTime},
		{str: "23:-01", err: ErrInvalidTime},
		{str: "24:00", err: ErrInvalidTime},
		{str: "-2:00", err: ErrInvalidTime},
	}
	for _, test := range testCases {
		minutes, err := MinutesAfterMidnight(test.str)
		if !errors.Is(err, test.err) {
			t.Errorf("expected '%s' to result in error %s, but got %s", test.str, test.err, err)
		} else if test.minutes != minutes {
			t.Errorf("expected MinutesAfterMidnight('%s') = %d, but got %d", test.str, test.minutes, minutes)
		}
	}
}

func TestGetDays(t *testing.T) {
	type params struct {
		str   string
		mask  int32
		isErr bool
	}
	testCases := []params{
		{str: "SUN", mask: 1},
		{str: "SUN, WED", mask: 1 + 8},
		{str: "SUN, WEDnesday", mask: 1 + 8},
		{str: "sUN,wEd", mask: 1 + 8},
		{str: "all", mask: 127},
		{str: "sun,all", mask: 127},
		{str: "mon,tues,wed,thurs", mask: 2 + 4 + 8 + 16},
		{str: "marketday", isErr: true},
		{str: "sun mon", isErr: true},
	}
	for _, test := range testCases {
		mask, err := GetDays(test.str)
		if (err != nil) != test.isErr {
			t.Errorf("day string '%s' gave unexpected err = %s", test.str, err)
		} else if mask != test.mask {
			t.Errorf("day string '%s' gave mask %s instead of %s", test.str, strconv.FormatInt(int64(mask), 2), strconv.FormatInt(int64(test.mask), 2))
		}
	}
}

func TestParseHomelinkDevice(t *testing.T) {
	t.Parallel()
	if got := parseHomelinkDevice(""); got.Index != nil || got.Name != "" {
		t.Errorf("empty selector = %+v, want zero value", got)
	}

	byIndex := parseHomelinkDevice("2")
	if byIndex.Index == nil || *byIndex.Index != 2 {
		t.Errorf("index selector = %+v, want Index=2", byIndex)
	}
	if byIndex.Name != "" {
		t.Errorf("index selector set Name %q", byIndex.Name)
	}

	zero := parseHomelinkDevice("0")
	if zero.Index == nil || *zero.Index != 0 {
		t.Errorf("index 0 = %+v, want explicit 0", zero)
	}

	byName := parseHomelinkDevice("Garage Right")
	if byName.Index != nil || byName.Name != "Garage Right" {
		t.Errorf("name selector = %+v, want Name=Garage Right", byName)
	}
}

func TestRenameKeyRequiresFleetAPI(t *testing.T) {
	t.Parallel()
	info, ok := commands["rename-key"]
	if !ok {
		t.Fatal("missing rename-key")
	}
	if !info.requiresFleetAPI {
		t.Error("rename-key must require Fleet API; Locks-screen names are not stored on the vehicle")
	}
	if info.requiresAuth {
		t.Error("rename-key talks to Tesla's account service, not a signed VCSEC session")
	}
}

func TestUpdateKeyWorksWithoutFleetAPI(t *testing.T) {
	t.Parallel()
	info, ok := commands["update-key"]
	if !ok {
		t.Fatal("missing update-key")
	}
	if info.requiresFleetAPI {
		t.Error("update-key must work over BLE")
	}
	if !info.requiresAuth {
		t.Error("update-key must be a signed whitelist operation")
	}
}

func TestFleetCommandBlockedByBLE(t *testing.T) {
	t.Parallel()
	if !errors.Is(fleetCommandBlockedByBLE("rename-key"), protocol.ErrKeyNameRequiresFleetAPI) {
		t.Fatal("rename-key over BLE must return ErrKeyNameRequiresFleetAPI")
	}
	if !errors.Is(fleetCommandBlockedByBLE("options"), protocol.ErrBatteryOptionRequiresFleetAPI) {
		t.Fatal("options over BLE must return ErrBatteryOptionRequiresFleetAPI")
	}
	if !errors.Is(fleetCommandBlockedByBLE("battery-option"), protocol.ErrBatteryOptionRequiresFleetAPI) {
		t.Fatal("battery-option over BLE must return ErrBatteryOptionRequiresFleetAPI")
	}
	if !errors.Is(fleetCommandBlockedByBLE("get"), ErrRequiresOAuth) {
		t.Fatal("other Fleet commands over BLE still need a generic OAuth error")
	}
}

func TestOptionsCommandsRequireFleetAPI(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"options", "battery-option"} {
		info, ok := commands[name]
		if !ok {
			t.Fatalf("missing %s", name)
		}
		if !info.requiresFleetAPI {
			t.Errorf("%s must require Fleet API; $BT* codes are not on the vehicle", name)
		}
		if info.requiresAuth {
			t.Errorf("%s talks to Tesla's catalog, not a signed vehicle session", name)
		}
	}
}

func TestWifiCommandReturnsProtocolError(t *testing.T) {
	t.Parallel()
	info, ok := commands["wifi"]
	if !ok {
		t.Fatal("missing wifi")
	}
	if info.requiresFleetAPI {
		t.Error("wifi help command must not require Fleet API")
	}
	if err := info.handler(context.Background(), nil, nil, nil); !errors.Is(err, protocol.ErrWiFiNotInProtocol) {
		t.Fatalf("wifi handler = %v, want ErrWiFiNotInProtocol", err)
	}
}

func TestKeepAwakeCommandReturnsProtocolError(t *testing.T) {
	t.Parallel()
	info, ok := commands["keep-awake"]
	if !ok {
		t.Fatal("missing keep-awake")
	}
	if info.requiresFleetAPI {
		t.Error("keep-awake help command must not require Fleet API")
	}
	if err := info.handler(context.Background(), nil, nil, nil); !errors.Is(err, protocol.ErrKeepAwakeNotInProtocol) {
		t.Fatalf("keep-awake handler = %v, want ErrKeepAwakeNotInProtocol", err)
	}
}

func TestChargingWhileAsleepCommandReturnsProtocolError(t *testing.T) {
	t.Parallel()
	info, ok := commands["charging-while-asleep"]
	if !ok {
		t.Fatal("missing charging-while-asleep")
	}
	if info.requiresFleetAPI || info.requiresAuth {
		t.Error("charging-while-asleep help command must not require a session")
	}
	if err := info.handler(context.Background(), nil, nil, nil); !errors.Is(err, protocol.ErrChargingWhileInfotainmentAsleep) {
		t.Fatalf("charging-while-asleep handler = %v, want ErrChargingWhileInfotainmentAsleep", err)
	}
}

func TestInfotainmentOfflineCommandReturnsProtocolError(t *testing.T) {
	t.Parallel()
	info, ok := commands["infotainment-offline"]
	if !ok {
		t.Fatal("missing infotainment-offline")
	}
	if info.requiresFleetAPI || info.requiresAuth {
		t.Error("infotainment-offline help command must not require a session")
	}
	if err := info.handler(context.Background(), nil, nil, nil); !errors.Is(err, protocol.ErrInfotainmentSignedCommandOffline) {
		t.Fatalf("infotainment-offline handler = %v, want ErrInfotainmentSignedCommandOffline", err)
	}
}

func TestBLEPresenceExemptCommandReturnsProtocolError(t *testing.T) {
	t.Parallel()
	info, ok := commands["ble-presence-exempt"]
	if !ok {
		t.Fatal("missing ble-presence-exempt")
	}
	if info.requiresFleetAPI {
		t.Error("ble-presence-exempt help command must not require Fleet API")
	}
	if err := info.handler(context.Background(), nil, nil, nil); !errors.Is(err, protocol.ErrBLEKeyPresenceNotInProtocol) {
		t.Fatalf("ble-presence-exempt handler = %v, want ErrBLEKeyPresenceNotInProtocol", err)
	}
}

func TestChargingScheduleOverheatCommandReturnsFirmwareError(t *testing.T) {
	t.Parallel()
	info, ok := commands["charging-schedule-overheat"]
	if !ok {
		t.Fatal("missing charging-schedule-overheat")
	}
	if info.requiresFleetAPI {
		t.Error("charging-schedule-overheat help command must not require Fleet API")
	}
	if err := info.handler(context.Background(), nil, nil, nil); !errors.Is(err, protocol.ErrScheduledChargingFirmware) {
		t.Fatalf("charging-schedule-overheat handler = %v, want ErrScheduledChargingFirmware", err)
	}
}

func TestBLEStateFastCommandReturnsFirmwareError(t *testing.T) {
	t.Parallel()
	info, ok := commands["ble-state-fast"]
	if !ok {
		t.Fatal("missing ble-state-fast")
	}
	if info.requiresFleetAPI {
		t.Error("ble-state-fast help command must not require Fleet API")
	}
	if err := info.handler(context.Background(), nil, nil, nil); !errors.Is(err, protocol.ErrBLEStateLatencyFirmware) {
		t.Fatalf("ble-state-fast handler = %v, want ErrBLEStateLatencyFirmware", err)
	}
}

func TestSeatHeaterNotImplementedCommandReturnsFleetAPIError(t *testing.T) {
	t.Parallel()
	info, ok := commands["seat-heater-not-implemented"]
	if !ok {
		t.Fatal("missing seat-heater-not-implemented")
	}
	if info.requiresFleetAPI {
		t.Error("seat-heater-not-implemented help command must not require Fleet API")
	}
	if err := info.handler(context.Background(), nil, nil, nil); !errors.Is(err, protocol.ErrSeatClimateFleetAPI) {
		t.Fatalf("seat-heater-not-implemented handler = %v, want ErrSeatClimateFleetAPI", err)
	}
}

func TestPartnerOAuthCommandReturnsProvisioningError(t *testing.T) {
	t.Parallel()
	info, ok := commands["partner-oauth"]
	if !ok {
		t.Fatal("missing partner-oauth")
	}
	if info.requiresFleetAPI || info.requiresAuth {
		t.Error("partner-oauth help command must not require a token Tesla has not issued")
	}
	if err := info.handler(context.Background(), nil, nil, nil); !errors.Is(err, protocol.ErrPartnerOAuthNotProvisioned) {
		t.Fatalf("partner-oauth handler = %v, want ErrPartnerOAuthNotProvisioned", err)
	}
}

func TestSeatHeaterAndCoolerCommandsAreImplemented(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"seat-heater", "seat-cooler"} {
		info, ok := commands[name]
		if !ok {
			t.Fatalf("missing %s", name)
		}
		if !info.requiresAuth {
			t.Errorf("%s must send a signed VehicleAction", name)
		}
		if len(info.args) != 2 {
			t.Errorf("%s args = %d, want SEAT and LEVEL", name, len(info.args))
		}
	}
}

func TestChargingManagerChargePortCommandReturnsFirmwareError(t *testing.T) {
	t.Parallel()
	info, ok := commands["charging-manager-charge-port"]
	if !ok {
		t.Fatal("missing charging-manager-charge-port")
	}
	if info.requiresFleetAPI {
		t.Error("charging-manager-charge-port help command must not require Fleet API")
	}
	if err := info.handler(context.Background(), nil, nil, nil); !errors.Is(err, protocol.ErrChargingManagerChargePortFirmware) {
		t.Fatalf("charging-manager-charge-port handler = %v, want ErrChargingManagerChargePortFirmware", err)
	}
}

func TestHvacAutoModeCommandReturnsProtocolError(t *testing.T) {
	t.Parallel()
	info, ok := commands["hvac-auto-mode"]
	if !ok {
		t.Fatal("missing hvac-auto-mode")
	}
	if info.requiresFleetAPI {
		t.Error("hvac-auto-mode help command must not require Fleet API")
	}
	if err := info.handler(context.Background(), nil, nil, nil); !errors.Is(err, protocol.ErrHvacAutoModeNotInProtocol) {
		t.Fatalf("hvac-auto-mode handler = %v, want ErrHvacAutoModeNotInProtocol", err)
	}
}

func TestClimateSplitCommandReturnsProtocolError(t *testing.T) {
	t.Parallel()
	info, ok := commands["climate-split"]
	if !ok {
		t.Fatal("missing climate-split")
	}
	if info.requiresFleetAPI || info.requiresAuth {
		t.Error("climate-split help command must not require a session")
	}
	if err := info.handler(context.Background(), nil, nil, nil); !errors.Is(err, protocol.ErrClimateSplitNotInProtocol) {
		t.Fatalf("climate-split handler = %v, want ErrClimateSplitNotInProtocol", err)
	}
}

func TestVirtualKeyReturnCommandRejectsRedirect(t *testing.T) {
	t.Parallel()
	info, ok := commands["virtual-key-return"]
	if !ok {
		t.Fatal("missing virtual-key-return")
	}
	if info.requiresFleetAPI || info.requiresAuth {
		t.Error("virtual-key-return must not require a session")
	}
	err := info.handler(context.Background(), nil, nil, map[string]string{
		"DOMAIN":     "example.com",
		"RETURN_URI": "https://evil.test/phish",
	})
	if !errors.Is(err, protocol.ErrVirtualKeyReturnURI) {
		t.Fatalf("virtual-key-return = %v, want ErrVirtualKeyReturnURI", err)
	}
	err = info.handler(context.Background(), nil, nil, map[string]string{
		"DOMAIN":     "example.com",
		"RETURN_URI": "https://example.com/finish-setup",
	})
	if !errors.Is(err, protocol.ErrVirtualKeyReturnURI) {
		t.Fatalf("same-domain return_uri = %v, want ErrVirtualKeyReturnURI", err)
	}
}

func TestVirtualKeyLinkCommandPrintsHostedURL(t *testing.T) {
	t.Parallel()
	info, ok := commands["virtual-key-link"]
	if !ok {
		t.Fatal("missing virtual-key-link")
	}
	if info.requiresFleetAPI || info.requiresAuth {
		t.Error("virtual-key-link must not require a session")
	}
	if err := info.handler(context.Background(), nil, nil, map[string]string{"DOMAIN": "example.com"}); err != nil {
		t.Fatalf("virtual-key-link: %v", err)
	}
	if err := info.handler(context.Background(), nil, nil, map[string]string{"DOMAIN": "https://evil.test"}); !errors.Is(err, protocol.ErrVirtualKeyReturnURI) {
		t.Fatalf("virtual-key-link bad domain = %v", err)
	}
}

func TestClimateKeeperCPDCommandReturnsFirmwareError(t *testing.T) {
	t.Parallel()
	info, ok := commands["climate-keeper-cpd"]
	if !ok {
		t.Fatal("missing climate-keeper-cpd")
	}
	if info.requiresFleetAPI || info.requiresAuth {
		t.Error("climate-keeper-cpd help command must not require a session")
	}
	if err := info.handler(context.Background(), nil, nil, nil); !errors.Is(err, protocol.ErrClimateKeeperCPDFirmware) {
		t.Fatalf("climate-keeper-cpd handler = %v, want ErrClimateKeeperCPDFirmware", err)
	}
}

func TestClimateKeeperCommandIsPublished(t *testing.T) {
	t.Parallel()
	info, ok := commands["climate-keeper"]
	if !ok {
		t.Fatal("missing climate-keeper")
	}
	if !info.requiresAuth || info.requiresFleetAPI {
		t.Error("climate-keeper must send a signed Infotainment command")
	}
	if len(info.args) != 1 || info.args[0].name != "MODE" {
		t.Fatal("climate-keeper must take MODE")
	}
}

func TestParseClimateKeeperMode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in      string
		want    vehicle.ClimateKeeperMode
		wantErr bool
	}{
		{"dog", vehicle.ClimateKeeperModeDog, false},
		{"2", vehicle.ClimateKeeperModeDog, false},
		{"camp", vehicle.ClimateKeeperModeCamp, false},
		{"3", vehicle.ClimateKeeperModeCamp, false},
		{"off", vehicle.ClimateKeeperModeOff, false},
		{"on", vehicle.ClimateKeeperModeOn, false},
		{"party", 0, true},
	}
	for _, tt := range tests {
		got, err := parseClimateKeeperMode(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Errorf("parseClimateKeeperMode(%q) = %v, want error", tt.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseClimateKeeperMode(%q): %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("parseClimateKeeperMode(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestBoomboxCommandReturnsProtocolError(t *testing.T) {
	t.Parallel()
	info, ok := commands["boombox"]
	if !ok {
		t.Fatal("missing boombox")
	}
	if info.requiresFleetAPI {
		t.Error("boombox help command must not require Fleet API")
	}
	if err := info.handler(context.Background(), nil, nil, nil); !errors.Is(err, protocol.ErrBoomboxNotInProtocol) {
		t.Fatalf("boombox handler = %v, want ErrBoomboxNotInProtocol", err)
	}
}
