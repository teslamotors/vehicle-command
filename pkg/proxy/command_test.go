package proxy_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/teslamotors/vehicle-command/pkg/connector/inet"
	"github.com/teslamotors/vehicle-command/pkg/protocol"
	"github.com/teslamotors/vehicle-command/pkg/proxy"
	"github.com/teslamotors/vehicle-command/pkg/vehicle"
)

func TestExtractCommandAction(t *testing.T) {
	ctx := context.Background()
	params := proxy.RequestParameters{
		"volume":        5.0,
		"on":            true,
		"seat_position": 0,
		"level":         2.0,
		// Add more test cases for different commands and parameters
	}

	tests := []struct {
		command      string
		params       proxy.RequestParameters
		expectedFunc func(*vehicle.Vehicle) error
		expected     error
	}{
		{"adjust_volume", params, func(v *vehicle.Vehicle) error { return v.SetVolume(ctx, 0.0) }, nil},
		{"adjust_volume", nil, nil, &protocol.NominalError{Details: fmt.Errorf("missing volume param")}},
		{"remote_boombox", params, nil, protocol.ErrBoomboxNotInProtocol},
		{"hvac_auto_mode", nil, nil, protocol.ErrHvacAutoModeNotInProtocol},
		{"set_hvac_auto", proxy.RequestParameters{"on": true}, nil, protocol.ErrHvacAutoModeNotInProtocol},
		{"climate_manual", nil, nil, protocol.ErrHvacAutoModeNotInProtocol},
		{"hvac_manual", nil, nil, protocol.ErrHvacAutoModeNotInProtocol},
		{"auto_hvac_mode", nil, nil, protocol.ErrHvacAutoModeNotInProtocol},
		{"climate_split", nil, nil, protocol.ErrClimateSplitNotInProtocol},
		{"set_climate_split", proxy.RequestParameters{"on": true}, nil, protocol.ErrClimateSplitNotInProtocol},
		{"climate_sync", proxy.RequestParameters{"on": false}, nil, protocol.ErrClimateSplitNotInProtocol},
		{"set_climate_sync", proxy.RequestParameters{"on": true}, nil, protocol.ErrClimateSplitNotInProtocol},
		{"virtual_key_return", proxy.RequestParameters{"return_uri": "https://evil.test/phish"}, nil, protocol.ErrVirtualKeyReturnURI},
		{"ak_return_uri", proxy.RequestParameters{"return_uri": "https://example.com/finish-setup"}, nil, protocol.ErrVirtualKeyReturnURI},
		{"set_virtual_key_return", nil, nil, protocol.ErrVirtualKeyReturnURI},
		{"climate_keeper_cpd", nil, nil, protocol.ErrClimateKeeperCPDFirmware},
		{"override_cpd", nil, nil, protocol.ErrClimateKeeperCPDFirmware},
		{"dog_mode_cpd", nil, nil, protocol.ErrClimateKeeperCPDFirmware},
		{"camp_mode_cpd", nil, nil, protocol.ErrClimateKeeperCPDFirmware},
		{"set_climate_keeper_mode", proxy.RequestParameters{"climate_keeper_mode": 2.0}, nil, nil},
		{"set_climate_keeper_mode", proxy.RequestParameters{"climate_keeper_mode": 3.0, "manual_override": true}, nil, nil},
		{"set_temps", nil, nil, &protocol.NominalError{Details: fmt.Errorf("missing driver_temp param")}},
		{"set_temps", proxy.RequestParameters{"driver_temp": 22.0}, nil, nil},
		{"set_temps", proxy.RequestParameters{"passenger_temp": 21.0}, nil, nil},
		{"set_temps", proxy.RequestParameters{"driver_temp": 22.0, "passenger_temp": 21.0}, nil, nil},
		{"auto_conditioning_start", nil, nil, nil},
		{"auto_conditioning_start", proxy.RequestParameters{"manual_override": true}, nil, nil},
		{"auto_conditioning_stop", nil, nil, nil},
		{"wifi_on", nil, nil, protocol.ErrWiFiNotInProtocol},
		{"wifi_off", nil, nil, protocol.ErrWiFiNotInProtocol},
		{"set_wifi", proxy.RequestParameters{"on": true}, nil, protocol.ErrWiFiNotInProtocol},
		{"add_wifi_network", proxy.RequestParameters{"ssid": "depot", "security": "wpa2", "psk": "secret"}, nil, protocol.ErrWiFiNotInProtocol},
		{"forget_wifi_network", proxy.RequestParameters{"ssid": "depot"}, nil, protocol.ErrWiFiNotInProtocol},
		{"wifi_connect_in_drive", proxy.RequestParameters{"on": true}, nil, protocol.ErrWiFiNotInProtocol},
		{"keep_awake", nil, nil, protocol.ErrKeepAwakeNotInProtocol},
		{"keep_alive", nil, nil, protocol.ErrKeepAwakeNotInProtocol},
		{"charging_while_asleep", nil, nil, protocol.ErrChargingWhileInfotainmentAsleep},
		{"charge_stop_asleep", nil, nil, protocol.ErrChargingWhileInfotainmentAsleep},
		{"set_charging_amps_asleep", nil, nil, protocol.ErrChargingWhileInfotainmentAsleep},
		{"infotainment_offline", nil, nil, protocol.ErrInfotainmentSignedCommandOffline},
		{"domain_infotainment_online", nil, nil, protocol.ErrInfotainmentSignedCommandOffline},
		{"vcsec_means_infotainment_online", nil, nil, protocol.ErrInfotainmentSignedCommandOffline},
		{"honk_horn", nil, nil, nil},
		{"charge_stop", nil, nil, nil},
		{"set_charging_amps", proxy.RequestParameters{"charging_amps": 16.0}, nil, nil},
		{"battery_size", nil, nil, protocol.ErrBatteryOptionRequiresFleetAPI},
		{"get_battery_option", nil, nil, protocol.ErrBatteryOptionRequiresFleetAPI},
		{"get_battery_size", nil, nil, protocol.ErrBatteryOptionRequiresFleetAPI},
		{"ble_presence_exempt", nil, nil, protocol.ErrBLEKeyPresenceNotInProtocol},
		{"command_only_key", nil, nil, protocol.ErrBLEKeyPresenceNotInProtocol},
		{"charging_manager_charge_port", nil, nil, protocol.ErrChargingManagerChargePortFirmware},
		{"grant_charging_manager_charge_port", nil, nil, protocol.ErrChargingManagerChargePortFirmware},
		{"charging_manager_port", nil, nil, protocol.ErrChargingManagerChargePortFirmware},
		{"ble_state_fast", nil, nil, protocol.ErrBLEStateLatencyFirmware},
		{"drive_state_fast", nil, nil, protocol.ErrBLEStateLatencyFirmware},
		{"set_ble_poll_interval", proxy.RequestParameters{"interval_ms": 50.0}, nil, protocol.ErrBLEStateLatencyFirmware},
		{"remote_seat_heater_request", proxy.RequestParameters{"seat_position": 0.0, "level": 2.0}, nil, nil},
		{"remote_seat_heater_request", proxy.RequestParameters{"heater": 1.0, "level": 3.0}, nil, nil},
		{"remote_seat_heater_request", proxy.RequestParameters{"level": 2.0}, nil, &protocol.NominalError{Details: fmt.Errorf("missing seat_position param")}},
		{"remote_seat_heater_request", proxy.RequestParameters{"heater": 99.0, "level": 1.0}, nil, errors.New("invalid seat position")},
		{"remote_seat_cooler_request", proxy.RequestParameters{"seat_position": 1.0, "seat_cooler_level": 2.0}, nil, nil},
		{"remote_seat_cooler_request", proxy.RequestParameters{"seat_position": 1.0, "level": 2.0}, nil, nil},
		{"remote_seat_cooler_request", proxy.RequestParameters{"seat_cooler_level": 2.0}, nil, &protocol.NominalError{Details: fmt.Errorf("missing seat_position param")}},
		{"seat_heater_not_implemented", nil, nil, protocol.ErrSeatClimateFleetAPI},
		{"seat_cooler_not_implemented", nil, nil, protocol.ErrSeatClimateFleetAPI},
		{"remote_seat_climate_not_implemented", nil, nil, protocol.ErrSeatClimateFleetAPI},
		{"partner_token", nil, nil, protocol.ErrPartnerOAuthNotProvisioned},
		{"register_partner", nil, nil, protocol.ErrPartnerOAuthNotProvisioned},
		{"oauth_audience", nil, nil, protocol.ErrPartnerOAuthNotProvisioned},
		{"invalid_audience", nil, nil, protocol.ErrPartnerOAuthNotProvisioned},
		{"charge_port_door_open", nil, nil, nil},
		{"charge_port_door_close", nil, nil, nil},
		{"scheduled_charging_overheat", nil, nil, protocol.ErrScheduledChargingFirmware},
		{"force_scheduled_charging", nil, nil, protocol.ErrScheduledChargingFirmware},
		{"invalid_command", params, nil, &inet.HTTPError{Code: http.StatusBadRequest, Message: "{\"response\":null,\"error\":\"invalid_command\",\"error_description\":\"\"}"}},
		{"trigger_homelink", proxy.RequestParameters{"lat": 37.5, "lon": -122.2}, nil, nil},
		{"trigger_homelink", proxy.RequestParameters{"lat": 37.5, "lon": -122.2, "homelink_device_index": 1.0}, nil, nil},
		{"trigger_homelink", proxy.RequestParameters{"lat": 37.5, "lon": -122.2, "homelink_device_index": 0.0}, nil, nil},
		{"trigger_homelink", proxy.RequestParameters{"lat": 37.5, "lon": -122.2, "homelink_device_name": "Garage Right"}, nil, nil},
		{"trigger_homelink", proxy.RequestParameters{"lat": 37.5}, nil, &protocol.NominalError{Details: fmt.Errorf("missing lon param")}},
		{"trigger_homelink", proxy.RequestParameters{"lat": 37.5, "lon": -122.2, "homelink_device_index": -1.0}, nil, &protocol.NominalError{Details: fmt.Errorf("invalid homelink_device_index param")}},
		{"trigger_homelink", proxy.RequestParameters{"lat": 37.5, "lon": -122.2, "homelink_device_index": 1.5}, nil, &protocol.NominalError{Details: fmt.Errorf("invalid homelink_device_index param")}},
		{"trigger_homelink", proxy.RequestParameters{"lat": 37.5, "lon": -122.2, "homelink_device_index": "1"}, nil, &protocol.NominalError{Details: fmt.Errorf("invalid homelink_device_index param")}},
		{"set_tent_mode", proxy.RequestParameters{"on": true}, nil, nil},
		{"set_tent_mode", proxy.RequestParameters{"on": false}, nil, nil},
		{"set_tent_mode", nil, nil, &protocol.NominalError{Details: fmt.Errorf("missing on param")}},
		{"set_tent_mode", proxy.RequestParameters{"on": "yes"}, nil, &protocol.NominalError{Details: fmt.Errorf("invalid on param")}},
		{"set_suspension_level", proxy.RequestParameters{"suspension_level": "medium"}, nil, nil},
		{"set_suspension_level", proxy.RequestParameters{"suspension_level": "level"}, nil, nil},
		{"set_suspension_level", proxy.RequestParameters{"suspension_level": 3.0}, nil, nil},
		{"set_suspension_level", proxy.RequestParameters{"suspension_level": "entry"}, nil, nil},
		{"set_suspension_level", nil, nil, &protocol.NominalError{Details: fmt.Errorf("missing suspension_level param")}},
		{"set_suspension_level", proxy.RequestParameters{"suspension_level": "park"}, nil, &protocol.NominalError{Details: fmt.Errorf("invalid suspension_level param")}},
		{"set_suspension_level", proxy.RequestParameters{"suspension_level": 0.0}, nil, &protocol.NominalError{Details: fmt.Errorf("invalid suspension_level param")}},
		{"level_suspension", nil, nil, nil},
	}

	for _, test := range tests {
		action, err := proxy.ExtractCommandAction(ctx, test.command, test.params)

		if errors.Is(err, test.expected) {
			if test.expected != nil && action != nil {

				t.Errorf("Expected error %#v but got action %p for command %#v", test.expected, action, test.command)
			}
		} else if err != nil && err.Error() != test.expected.Error() {
			t.Errorf("Unexpected error for command %s: %v", test.command, err)
		}
	}
}

func TestRemoteSeatHeaterIsImplemented(t *testing.T) {
	ctx := context.Background()
	action, err := proxy.ExtractCommandAction(ctx, "remote_seat_heater_request", proxy.RequestParameters{
		"heater": 0.0,
		"level":  2.0,
	})
	if err != nil {
		t.Fatalf("Owner API heater alias must parse: %v", err)
	}
	if action == nil {
		t.Fatal("remote_seat_heater_request must return an action, not ErrCommandNotImplemented")
	}
	if errors.Is(err, protocol.ErrSeatClimateFleetAPI) {
		t.Fatal("the published heater path must send HvacSeatHeaterActions")
	}

	action, err = proxy.ExtractCommandAction(ctx, "remote_seat_cooler_request", proxy.RequestParameters{
		"seat_position":     1.0,
		"seat_cooler_level": 2.0,
	})
	if err != nil {
		t.Fatalf("remote_seat_cooler_request must parse: %v", err)
	}
	if action == nil {
		t.Fatal("remote_seat_cooler_request must return an action")
	}

	_, err = proxy.ExtractCommandAction(ctx, "remote_seat_heater_request", proxy.RequestParameters{
		"heater": 99.0,
		"level":  1.0,
	})
	if err == nil || err.Error() != "invalid seat position" {
		t.Fatalf("invalid heater index = %v, want invalid seat position", err)
	}
}
