package proxy

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/teslamotors/vehicle-command/pkg/connector/inet"
	"github.com/teslamotors/vehicle-command/pkg/protocol"
	"github.com/teslamotors/vehicle-command/pkg/vehicle"

	carserver "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/carserver"
)

var (
	// ErrCommandNotImplemented indicates a command has not be implemented in the SDK.
	// remote_boombox is not this case: Tesla has not published a VehicleAction.
	// Use protocol.ErrBoomboxNotInProtocol. Tesla has not published the
	// VehicleAction pending legal review (teslamotors/vehicle-command#266).
	ErrCommandNotImplemented = errors.New("command not implemented")

	// ErrCommandUseRESTAPI indicates vehicle/command is not supported by the protocol
	ErrCommandUseRESTAPI = errors.New("command requires using the REST API")

	seatPositions = []vehicle.SeatPosition{
		vehicle.SeatFrontLeft,
		vehicle.SeatFrontRight,
		vehicle.SeatSecondRowLeft,
		vehicle.SeatSecondRowLeftBack,
		vehicle.SeatSecondRowCenter,
		vehicle.SeatSecondRowRight,
		vehicle.SeatSecondRowRightBack,
		vehicle.SeatThirdRowLeft,
		vehicle.SeatThirdRowRight,
	}

	dayNamesBitMask = map[string]int32{
		"SUN":       1,
		"SUNDAY":    1,
		"MON":       2,
		"MONDAY":    2,
		"TUES":      4,
		"TUESDAY":   4,
		"WED":       8,
		"WEDNESDAY": 8,
		"THURS":     16,
		"THURSDAY":  16,
		"FRI":       32,
		"FRIDAY":    32,
		"SAT":       64,
		"SATURDAY":  64,
		"ALL":       127,
		"WEEKDAYS":  62,
	}
)

// RequestParameters allows simple type check
type RequestParameters map[string]interface{}

// ExtractCommandAction use command to define which action should be executed.
func ExtractCommandAction(ctx context.Context, command string, params RequestParameters) (func(*vehicle.Vehicle) error, error) {
	switch command {
	// Media controls
	case "adjust_volume":
		volume, err := params.getNumber("volume", true)
		if err != nil {
			return nil, err
		}
		return func(v *vehicle.Vehicle) error { return v.SetVolume(ctx, float32(volume)) }, nil
	case "remote_boombox":
		// Fleet API lists this REST path; Tesla has not published a
		// VehicleAction. Legal has not cleared Pedestrian Warning System
		// use (teslamotors/vehicle-command#266). Do not invent a field,
		// copy a firmware dump, or map it to honk.
		return nil, protocol.ErrBoomboxNotInProtocol
	case "wifi_on", "wifi_off", "set_wifi", "add_wifi_network", "forget_wifi_network", "wifi_connect_in_drive":
		// In-car UX exists; Tesla has not published VehicleAction numbers. Do not
		// invent fields or send a PSK. teslamotors/vehicle-command#419.
		return nil, protocol.ErrWiFiNotInProtocol
	case "hvac_auto_mode", "set_hvac_auto", "climate_manual", "hvac_manual", "auto_hvac_mode":
		// HvacAutoAction is climate power, not Auto vs Manual HVAC.
		// teslamotors/vehicle-command#283.
		return nil, protocol.ErrHvacAutoModeNotInProtocol
	case "climate_split", "set_climate_split", "climate_sync", "set_climate_sync":
		// In-car SYNC/split is unpublished. Independent temps are set_temps.
		// teslamotors/vehicle-command#386.
		return nil, protocol.ErrClimateSplitNotInProtocol
	case "virtual_key_return", "ak_return_uri", "set_virtual_key_return":
		// https://tesla.com/_ak/<domain> is Tesla's hosted page. An
		// unconstrained return_uri is an open redirect. teslamotors/vehicle-command#444.
		return nil, protocol.ErrVirtualKeyReturnURI
	case "climate_keeper_cpd", "override_cpd", "dog_mode_cpd", "camp_mode_cpd":
		// set_climate_keeper_mode is published. Firmware may refuse Dog/Camp
		// with cpd_enabled. Do not invent a CPD bypass.
		// teslamotors/vehicle-command#509.
		return nil, protocol.ErrClimateKeeperCPDFirmware
	case "keep_awake", "keep_alive":
		// wake does not inhibit sleep; Tesla has not published a keep-awake
		// VehicleAction. Do not wrap charge-port-close. teslamotors/vehicle-command#397.
		return nil, protocol.ErrKeepAwakeNotInProtocol
	case "charging_while_asleep", "charge_stop_asleep", "set_charging_amps_asleep":
		// charge_stop / set_charging_amps are published Infotainment actions.
		// Fleet Telemetry can look live while Tesla signed_command returns
		// vehicle unavailable. Do not invent keep-awake.
		// teslamotors/vehicle-command#452.
		return nil, protocol.ErrChargingWhileInfotainmentAsleep
	case "infotainment_offline", "domain_infotainment_online", "vcsec_means_infotainment_online":
		// state=online and a VCSEC signed_command do not prove
		// DOMAIN_INFOTAINMENT will be accepted. Do not rewrite the domain
		// or skip that handshake. teslamotors/vehicle-command#285.
		return nil, protocol.ErrInfotainmentSignedCommandOffline
	case "battery_size", "get_battery_option", "get_battery_size":
		// Pack identity is Fleet DX catalog metadata, not a signed command.
		// teslamotors/vehicle-command#391.
		return nil, protocol.ErrBatteryOptionRequiresFleetAPI
	case "ble_presence_exempt", "command_only_key":
		// Enrolled BLE clients are VCSEC whitelist keys. Tesla has not
		// published a flag that ignores them for Walk-Away Door Lock.
		// teslamotors/vehicle-command#480.
		return nil, protocol.ErrBLEKeyPresenceNotInProtocol
	case "charging_manager_charge_port", "grant_charging_manager_charge_port", "charging_manager_port":
		// ChargePortDoorOpen/Close are published. Charging Manager ACL is firmware.
		// Do not enroll Owner or bypass via VCSEC closures. teslamotors/vehicle-command#413.
		return nil, protocol.ErrChargingManagerChargePortFirmware
	case "ble_state_fast", "drive_state_fast", "set_ble_poll_interval":
		// GetState latency is the vehicle BLE round-trip. Do not disable
		// encryption or invent a streaming DriveState. teslamotors/vehicle-command#414.
		return nil, protocol.ErrBLEStateLatencyFirmware
	case "seat_heater_not_implemented", "seat_cooler_not_implemented", "remote_seat_climate_not_implemented":
		// remote_seat_heater_request / remote_seat_cooler_request already map
		// to published HvacSeatHeaterActions / HvacSeatCoolerActions.
		// Tesla signed_command HTTP 501 Unauthorized is Fleet API, not a
		// missing handler. teslamotors/vehicle-command#383.
		return nil, protocol.ErrSeatClimateFleetAPI
	case "partner_token", "register_partner", "oauth_audience", "invalid_audience":
		// Tesla Fleet Auth invalid_audience /authorize "No policy rules" is
		// Tesla IdP provisioning, not a missing VehicleAction.
		// teslamotors/vehicle-command#460.
		return nil, protocol.ErrPartnerOAuthNotProvisioned
	case "scheduled_charging_overheat", "force_scheduled_charging":
		// Schedule commands are published; later sleep/wake is firmware.
		// Do not disable cabin overheat as a workaround.
		// teslamotors/vehicle-command#342.
		return nil, protocol.ErrScheduledChargingFirmware
	case "media_next_fav":
		return func(v *vehicle.Vehicle) error { return v.MediaNextFavorite(ctx) }, nil
	case "media_prev_fav":
		return func(v *vehicle.Vehicle) error { return v.MediaPreviousFavorite(ctx) }, nil
	case "media_next_track":
		return func(v *vehicle.Vehicle) error { return v.MediaNextTrack(ctx) }, nil
	case "media_prev_track":
		return func(v *vehicle.Vehicle) error { return v.MediaPreviousTrack(ctx) }, nil
	case "media_volume_down":
		return func(v *vehicle.Vehicle) error { return v.VolumeDown(ctx) }, nil
	case "media_volume_up":
		return func(v *vehicle.Vehicle) error { return v.VolumeUp(ctx) }, nil
	case "media_toggle_playback":
		return func(v *vehicle.Vehicle) error { return v.ToggleMediaPlayback(ctx) }, nil
	// Climate Controls
	case "auto_conditioning_start":
		override, err := params.getBool("manual_override", false)
		if err != nil {
			return nil, err
		}
		return func(v *vehicle.Vehicle) error { return v.SetClimatePower(ctx, true, override) }, nil
	case "auto_conditioning_stop":
		return func(v *vehicle.Vehicle) error { return v.ClimateOff(ctx) }, nil
	case "charge_max_range":
		return func(v *vehicle.Vehicle) error { return v.ChargeMaxRange(ctx) }, nil
	case "remote_seat_cooler_request":
		level, seat, err := params.settingForCoolerSeatPosition()
		if err != nil {
			return nil, err
		}
		return func(v *vehicle.Vehicle) error { return v.SetSeatCooler(ctx, level, seat) }, nil
	case "remote_seat_heater_request":
		// Published HvacSeatHeaterActions. Owner/Fleet JSON "heater" is an
		// alias for "seat_position". Tesla signed_command HTTP 501 is Fleet
		// API, not a missing handler. teslamotors/vehicle-command#383.
		setting, err := params.settingForHeatSeatPosition()
		if err != nil {
			return nil, err
		}

		return func(v *vehicle.Vehicle) error { return v.SetSeatHeater(ctx, setting) }, nil
	case "remote_auto_seat_climate_request":
		seat, enabled, err := params.settingForAutoSeatPosition()
		if err != nil {
			return nil, err
		}
		return func(v *vehicle.Vehicle) error {
			return v.AutoSeatAndClimate(ctx, []vehicle.SeatPosition{seat}, enabled)
		}, nil
	case "remote_steering_wheel_heater_request":
		on, err := params.getBool("on", true)
		if err != nil {
			return nil, err
		}
		return func(v *vehicle.Vehicle) error { return v.SetSteeringWheelHeater(ctx, on) }, nil
	case "set_bioweapon_mode":
		on, err := params.getBool("on", true)
		if err != nil {
			return nil, err
		}
		override, err := params.getBool("manual_override", true)
		if err != nil {
			return nil, err
		}
		return func(v *vehicle.Vehicle) error { return v.SetBioweaponDefenseMode(ctx, on, override) }, nil
	case "set_cabin_overheat_protection":
		on, err := params.getBool("on", true)
		if err != nil {
			return nil, err
		}
		fanOnly, err := params.getBool("fan_only", false)
		if err != nil {
			return nil, err
		}
		return func(v *vehicle.Vehicle) error { return v.SetCabinOverheatProtection(ctx, on, fanOnly) }, nil
	case "set_climate_keeper_mode":
		// HvacClimateKeeperAction. Firmware may refuse Dog/Camp with
		// NominalError cpd_enabled. teslamotors/vehicle-command#509.
		// 0 : off
		// 1 : On
		// 2 : Dog
		// 3 : Camp
		mode, err := params.getNumber("climate_keeper_mode", true)
		if err != nil {
			return nil, err
		}
		override, err := params.getBool("manual_override", false)
		if err != nil {
			return nil, err
		}
		return func(v *vehicle.Vehicle) error {
			return v.SetClimateKeeperMode(ctx, vehicle.ClimateKeeperMode(mode), override)
		}, nil
	case "set_cop_temp":
		level, err := params.getNumber("cop_temp", true)
		if err != nil {
			return nil, err
		}
		return func(v *vehicle.Vehicle) error {
			return v.SetCabinOverheatProtectionTemperature(ctx, vehicle.Level(level))
		}, nil
	case "set_preconditioning_max":
		on, err := params.getBool("on", true)
		if err != nil {
			return nil, err
		}
		override, err := params.getBool("manual_override", false)
		if err != nil {
			return nil, err
		}
		return func(v *vehicle.Vehicle) error { return v.SetPreconditioningMax(ctx, on, override) }, nil
	case "set_temps":
		_, hasDriver := params["driver_temp"]
		_, hasPassenger := params["passenger_temp"]
		if !hasDriver && !hasPassenger {
			return nil, missingParamError("driver_temp")
		}
		driverTemp, err := params.getNumber("driver_temp", hasDriver)
		if err != nil {
			return nil, err
		}
		passengerTemp, err := params.getNumber("passenger_temp", hasPassenger)
		if err != nil {
			return nil, err
		}
		if !hasDriver {
			driverTemp = passengerTemp
		}
		if !hasPassenger {
			passengerTemp = driverTemp
		}
		return func(v *vehicle.Vehicle) error {
			return v.ChangeClimateTemp(ctx, float32(driverTemp), float32(passengerTemp))
		}, nil
	// vehicle.Vehicle actuation commands
	case "actuate_trunk":
		if which, err := params.getString("which_trunk", false); err == nil {
			switch which {
			case "front":
				return func(v *vehicle.Vehicle) error { return v.OpenFrunk(ctx) }, nil
			case "rear":
				return func(v *vehicle.Vehicle) error { return v.OpenTrunk(ctx) }, nil
			default:
				return nil, &protocol.NominalError{Details: protocol.NewError("invalid_value", false, false)}
			}
		}
		return func(v *vehicle.Vehicle) error { return v.OpenTrunk(ctx) }, nil
	case "charge_port_door_open":
		return func(v *vehicle.Vehicle) error { return v.ChargePortOpen(ctx) }, nil
	case "charge_port_door_close":
		return func(v *vehicle.Vehicle) error { return v.ChargePortClose(ctx) }, nil
	case "flash_lights":
		return func(v *vehicle.Vehicle) error { return v.FlashLights(ctx) }, nil
	case "honk_horn":
		return func(v *vehicle.Vehicle) error { return v.HonkHorn(ctx) }, nil
	case "remote_start_drive":
		return func(v *vehicle.Vehicle) error { return v.RemoteDrive(ctx) }, nil
	case "open_tonneau":
		return func(v *vehicle.Vehicle) error { return v.OpenTonneau(ctx) }, nil
	case "close_tonneau":
		return func(v *vehicle.Vehicle) error { return v.CloseTonneau(ctx) }, nil
	case "stop_tonneau":
		return func(v *vehicle.Vehicle) error { return v.StopTonneau(ctx) }, nil
	// Power-management controls
	case "set_low_power_mode":
		on, err := params.getBool("enable", true)
		if err != nil {
			return nil, err
		}
		return func(v *vehicle.Vehicle) error { return v.SetLowPowerMode(ctx, on) }, nil
	case "keep_accessory_power_mode":
		on, err := params.getBool("enable", true)
		if err != nil {
			return nil, err
		}
		return func(v *vehicle.Vehicle) error { return v.SetKeepAccessoryPowerMode(ctx, on) }, nil
	case "set_tent_mode":
		on, err := params.getBool("on", true)
		if err != nil {
			return nil, err
		}
		return func(v *vehicle.Vehicle) error { return v.SetTentMode(ctx, on) }, nil
	case "set_suspension_level":
		level, err := params.suspensionLevel()
		if err != nil {
			return nil, err
		}
		return func(v *vehicle.Vehicle) error { return v.SetSuspensionLevel(ctx, level) }, nil
	case "level_suspension":
		return func(v *vehicle.Vehicle) error { return v.LevelSuspension(ctx) }, nil
	case "charge_standard":
		return func(v *vehicle.Vehicle) error { return v.ChargeStandardRange(ctx) }, nil
	case "charge_start":
		return func(v *vehicle.Vehicle) error { return v.ChargeStart(ctx) }, nil
	case "charge_stop":
		// ChargingStartStopAction stop. Tesla may return
		// inet.ErrVehicleNotAwake while Fleet Telemetry still shows charging.
		// teslamotors/vehicle-command#452.
		return func(v *vehicle.Vehicle) error { return v.ChargeStop(ctx) }, nil
	case "set_charging_amps":
		amps, err := params.getNumber("charging_amps", true)
		if err != nil {
			return nil, err
		}
		return func(v *vehicle.Vehicle) error { return v.SetChargingAmps(ctx, int32(amps)) }, nil
	case "set_scheduled_charging":
		on, err := params.getBool("enable", true)
		if err != nil {
			return nil, err
		}
		scheduledTime, err := params.getTimeAfterMidnight("time")
		if err != nil {
			return nil, err
		}
		return func(v *vehicle.Vehicle) error { return v.ScheduleCharging(ctx, on, scheduledTime) }, nil
	case "set_charge_limit":
		limit, err := params.getNumber("percent", true)
		if err != nil {
			return nil, err
		}
		return func(v *vehicle.Vehicle) error { return v.ChangeChargeLimit(ctx, int32(limit)) }, nil
	case "set_scheduled_departure":
		enable, err := params.getBool("enable", true)
		if err != nil {
			return nil, err
		}
		if !enable {
			return func(v *vehicle.Vehicle) error { return v.ClearScheduledDeparture(ctx) }, nil
		}

		offPeakPolicy, err := params.getPolicy("off_peak_charging_enabled", "off_peak_charging_weekdays_only")
		if err != nil {
			return nil, err
		}
		preconditionPolicy, err := params.getPolicy("preconditioning_enabled", "preconditioning_weekdays_only")
		if err != nil {
			return nil, err
		}

		departureTime, err := params.getTimeAfterMidnight("departure_time")
		if err != nil {
			return nil, err
		}
		endOffPeakTime, err := params.getTimeAfterMidnight("end_off_peak_time")
		if err != nil {
			return nil, err
		}
		return func(v *vehicle.Vehicle) error {
			return v.ScheduleDeparture(ctx, departureTime, endOffPeakTime, preconditionPolicy, offPeakPolicy)
		}, nil
	case "add_charge_schedule":
		lat, err := params.getNumber("lat", true)
		if err != nil {
			return nil, err
		}
		lon, err := params.getNumber("lon", true)
		if err != nil {
			return nil, err
		}
		startTime, err := params.getNumber("start_time", false)
		if err != nil {
			return nil, err
		}
		startEnabled, err := params.getBool("start_enabled", true)
		if err != nil {
			return nil, err
		}
		endTime, err := params.getNumber("end_time", false)
		if err != nil {
			return nil, err
		}
		endEnabled, err := params.getBool("end_enabled", true)
		if err != nil {
			return nil, err
		}
		daysOfWeek, err := params.getDays("days_of_week", true)
		if err != nil {
			return nil, err
		}
		id, err := params.getNumber("id", false)
		if err != nil {
			return nil, err
		}
		idUint64 := uint64(id)
		if id == 0 {
			idUint64 = uint64(time.Now().Unix())
		}
		enabled, err := params.getBool("enabled", true)
		if err != nil {
			return nil, err
		}
		oneTime, err := params.getBool("one_time", false)
		if err != nil {
			return nil, err
		}
		schedule := vehicle.ChargeSchedule{
			DaysOfWeek:   daysOfWeek,
			Latitude:     float32(lat),
			Longitude:    float32(lon),
			Id:           idUint64,
			StartTime:    int32(startTime),
			EndTime:      int32(endTime),
			StartEnabled: startEnabled,
			EndEnabled:   endEnabled,
			Enabled:      enabled,
			OneTime:      oneTime,
		}
		return func(v *vehicle.Vehicle) error { return v.AddChargeSchedule(ctx, &schedule) }, nil
	case "add_precondition_schedule":
		lat, err := params.getNumber("lat", true)
		if err != nil {
			return nil, err
		}
		lon, err := params.getNumber("lon", true)
		if err != nil {
			return nil, err
		}
		preconditionTime, err := params.getNumber("precondition_time", true)
		if err != nil {
			return nil, err
		}
		oneTime, err := params.getBool("one_time", false)
		if err != nil {
			return nil, err
		}
		daysOfWeek, err := params.getDays("days_of_week", true)
		if err != nil {
			return nil, err
		}
		id, err := params.getNumber("id", false)
		if err != nil {
			return nil, err
		}
		idUint64 := uint64(id)
		if id == 0 {
			idUint64 = uint64(time.Now().Unix())
		}
		enabled, err := params.getBool("enabled", true)
		if err != nil {
			return nil, err
		}
		schedule := vehicle.PreconditionSchedule{
			DaysOfWeek:       daysOfWeek,
			Latitude:         float32(lat),
			Longitude:        float32(lon),
			Id:               idUint64,
			PreconditionTime: int32(preconditionTime),
			OneTime:          oneTime,
			Enabled:          enabled,
		}
		return func(v *vehicle.Vehicle) error { return v.AddPreconditionSchedule(ctx, &schedule) }, nil
	case "remove_charge_schedule":
		id, err := params.getNumber("id", true)
		if err != nil {
			return nil, err
		}
		return func(v *vehicle.Vehicle) error { return v.RemoveChargeSchedule(ctx, uint64(id)) }, nil
	case "remove_precondition_schedule":
		id, err := params.getNumber("id", true)
		if err != nil {
			return nil, err
		}
		return func(v *vehicle.Vehicle) error { return v.RemovePreconditionSchedule(ctx, uint64(id)) }, nil
	case "set_managed_charge_current_request":
		return nil, ErrCommandUseRESTAPI
	case "set_managed_charger_location":
		return nil, ErrCommandUseRESTAPI
	case "set_managed_scheduled_charging_time":
		return nil, ErrCommandUseRESTAPI
	case "wake_up":
		return func(v *vehicle.Vehicle) error { return v.Wakeup(ctx) }, nil
	// Security
	case "set_pin_to_drive":
		on, err := params.getBool("on", true)
		if err != nil {
			return nil, err
		}
		password, err := params.getString("password", false)
		if err != nil {
			return nil, err
		}
		return func(v *vehicle.Vehicle) error { return v.SetPINToDrive(ctx, on, password) }, nil
	case "clear_pin_to_drive_admin":
		return func(v *vehicle.Vehicle) error { return v.ClearPINToDrive(ctx) }, nil
	case "door_lock":
		return func(v *vehicle.Vehicle) error { return v.Lock(ctx) }, nil
	case "door_unlock":
		return func(v *vehicle.Vehicle) error { return v.Unlock(ctx) }, nil
	case "erase_user_data":
		return func(v *vehicle.Vehicle) error { return v.EraseGuestData(ctx) }, nil
	case "reset_pin_to_drive_pin":
		return func(v *vehicle.Vehicle) error { return v.ResetPIN(ctx) }, nil //nolint:all
	case "reset_valet_pin":
		return func(v *vehicle.Vehicle) error { return v.ResetValetPin(ctx) }, nil
	case "guest_mode":
		on, err := params.getBool("enable", true)
		if err != nil {
			return nil, err
		}
		return func(v *vehicle.Vehicle) error { return v.SetGuestMode(ctx, on) }, nil
	case "set_sentry_mode":
		on, err := params.getBool("on", true)
		if err != nil {
			return nil, err
		}
		return func(v *vehicle.Vehicle) error { return v.SetSentryMode(ctx, on) }, nil
	case "set_valet_mode":
		on, err := params.getBool("on", true)
		if err != nil {
			return nil, err
		}
		password, err := params.getString("password", false)
		if err != nil {
			return nil, err
		}
		if on {
			return func(v *vehicle.Vehicle) error { return v.EnableValetMode(ctx, password) }, nil
		}
		return func(v *vehicle.Vehicle) error { return v.DisableValetMode(ctx) }, nil
	case "set_vehicle_name":
		name, err := params.getString("vehicle_name", true)
		if err != nil {
			return nil, err
		}
		return func(v *vehicle.Vehicle) error { return v.SetVehicleName(ctx, name) }, nil
	case "speed_limit_activate":
		pin, err := params.getString("pin", true)
		if err != nil {
			return nil, err
		}
		return func(v *vehicle.Vehicle) error { return v.ActivateSpeedLimit(ctx, pin) }, nil
	case "speed_limit_deactivate":
		pin, err := params.getString("pin", true)
		if err != nil {
			return nil, err
		}
		return func(v *vehicle.Vehicle) error { return v.DeactivateSpeedLimit(ctx, pin) }, nil
	case "speed_limit_clear_pin":
		pin, err := params.getString("pin", true)
		if err != nil {
			return nil, err
		}
		return func(v *vehicle.Vehicle) error { return v.ClearSpeedLimitPIN(ctx, pin) }, nil
	case "speed_limit_clear_pin_admin":
		return func(v *vehicle.Vehicle) error { return v.ClearSpeedLimitPINAdminAction(ctx) }, nil
	case "speed_limit_set_limit":
		speedMPH, err := params.getNumber("limit_mph", true)
		if err != nil {
			return nil, err
		}
		return func(v *vehicle.Vehicle) error { return v.SpeedLimitSetLimitMPH(ctx, speedMPH) }, nil
	case "parental_controls_activate":
		pin, err := params.getString("pin", true)
		if err != nil {
			return nil, err
		}
		return func(v *vehicle.Vehicle) error { return v.ParentalControlsActivate(ctx, pin) }, nil
	case "parental_controls_deactivate":
		pin, err := params.getString("pin", true)
		if err != nil {
			return nil, err
		}
		return func(v *vehicle.Vehicle) error { return v.ParentalControlsDeactivate(ctx, pin) }, nil
	case "parental_controls_enable_setting":
		settingStr, err := params.getString("setting", true)
		if err != nil {
			return nil, err
		}
		settingVal, ok := carserver.ParentalControlsEnableSettingsAction_ParentalControlsSetting_E_value[settingStr]
		if !ok {
			return nil, &protocol.NominalError{Details: fmt.Errorf("invalid setting: %s", settingStr)}
		}
		enable, err := params.getBool("enable", true)
		if err != nil {
			return nil, err
		}
		return func(v *vehicle.Vehicle) error {
			return v.ParentalControlsEnableSetting(ctx, vehicle.ParentalControlsSetting(settingVal), enable)
		}, nil
	case "parental_controls_set_speed_limit":
		limitMPH, err := params.getNumber("limit_mph", true)
		if err != nil {
			return nil, err
		}
		return func(v *vehicle.Vehicle) error { return v.ParentalControlsSetSpeedLimit(ctx, limitMPH) }, nil
	case "parental_controls_clear_pin_admin":
		return func(v *vehicle.Vehicle) error { return v.ParentalControlsClearPIN(ctx) }, nil
	case "trigger_homelink":
		lat, err := params.getNumber("lat", true)
		if err != nil {
			return nil, err
		}
		lon, err := params.getNumber("lon", true)
		if err != nil {
			return nil, err
		}
		device, err := params.homelinkDevice()
		if err != nil {
			return nil, err
		}
		return func(v *vehicle.Vehicle) error {
			return v.TriggerHomelinkDevice(ctx, float32(lat), float32(lon), device)
		}, nil
	// Updates
	case "schedule_software_update":
		offsetSeconds, err := params.getNumber("offset_sec", true)
		if err != nil {
			return nil, err
		}
		return func(v *vehicle.Vehicle) error {
			return v.ScheduleSoftwareUpdate(ctx, time.Duration(offsetSeconds)*time.Second)
		}, nil
	case "cancel_software_update":
		return func(v *vehicle.Vehicle) error { return v.CancelSoftwareUpdate(ctx) }, nil
	// Sharing options. These endpoints often require server-side processing, which prevents strict
	// end-to-end authentication.
	case "navigation_request":
		return nil, ErrCommandUseRESTAPI
	case "window_control":
		// Latitude and longitude are not required for vehicles that support this protocol.
		cmd, err := params.getString("command", true)
		if err != nil {
			return nil, err
		}
		switch cmd {
		case "vent":
			return func(v *vehicle.Vehicle) error { return v.VentWindows(ctx) }, nil
		case "close":
			return func(v *vehicle.Vehicle) error { return v.CloseWindows(ctx) }, nil
		default:
			return nil, errors.New("command must be 'vent' or 'close'")
		}
	default:
		return nil, &inet.HTTPError{Code: http.StatusBadRequest, Message: "{\"response\":null,\"error\":\"invalid_command\",\"error_description\":\"\"}"}
	}
}

func (p RequestParameters) getString(key string, required bool) (string, error) {
	value, exists := p[key]

	if exists {
		if strValue, isString := value.(string); isString {
			return strValue, nil
		}
		return "", invalidParamError(key)
	}

	if !required {
		return "", nil
	}

	return "", missingParamError(key)
}

func (p RequestParameters) getBool(key string, required bool) (bool, error) {
	value, exists := p[key]
	if exists {
		if val, isBool := value.(bool); isBool {
			return val, nil
		}
		return false, invalidParamError(key)
	}

	if !required {
		return false, nil
	}

	return false, missingParamError(key)
}

func (p RequestParameters) getNumber(key string, required bool) (float64, error) {
	value, exists := p[key]
	if exists {
		if num, isFloat64 := value.(float64); isFloat64 {
			return num, nil
		}
		return 0, invalidParamError(key)
	}

	if !required {
		return 0, nil
	}

	return 0, missingParamError(key)
}

// getNumberAny returns the first present key among keys. The first key is
// used in missing-param errors. Owner API remote_seat_heater_request uses
// "heater"; tesla-http-proxy historically documented "seat_position".
func (p RequestParameters) getNumberAny(required bool, keys ...string) (float64, error) {
	for _, key := range keys {
		if _, exists := p[key]; exists {
			return p.getNumber(key, true)
		}
	}
	if !required {
		return 0, nil
	}
	if len(keys) == 0 {
		return 0, missingParamError("number")
	}
	return 0, missingParamError(keys[0])
}

func (p RequestParameters) getDays(key string, required bool) (int32, error) {
	daysStr, err := p.getString(key, required)
	if err != nil {
		return 0, err
	}

	var mask int32
	for _, d := range strings.Split(daysStr, ",") {
		if v, ok := dayNamesBitMask[strings.TrimSpace(strings.ToUpper(d))]; ok {
			mask |= v
		} else {
			return 0, fmt.Errorf("unrecognized day name: %v", d)
		}
	}
	return mask, nil
}

func (p RequestParameters) getPolicy(enabledKey string, weekdaysOnlyKey string) (vehicle.ChargingPolicy, error) {
	enabled, err := p.getBool(enabledKey, false)
	if err != nil {
		return 0, err
	}
	weekdaysOnly, err := p.getBool(weekdaysOnlyKey, false)
	if err != nil {
		return 0, err
	}
	if weekdaysOnly {
		return vehicle.ChargingPolicyWeekdays, nil
	}
	if enabled {
		return vehicle.ChargingPolicyAllDays, nil
	}
	return vehicle.ChargingPolicyOff, nil
}

func (p RequestParameters) getTimeAfterMidnight(key string) (time.Duration, error) {
	minutes, err := p.getNumber(key, false)
	if err != nil {
		return 0, err
	}
	// Leave further validation to the car for consistency with previous API.
	return time.Duration(minutes) * time.Minute, nil
}

func (p RequestParameters) settingForHeatSeatPosition() (map[vehicle.SeatPosition]vehicle.Level, error) {
	// Fleet/Owner API documents "heater"; tesla-http-proxy uses "seat_position"
	// (teslamotors/vehicle-command#133, #383). Prefer seat_position when both are set.
	index, err := p.getNumberAny(true, "seat_position", "heater")
	if err != nil {
		return nil, err
	}
	if int(index) < 0 || int(index) >= len(seatPositions) {
		return nil, errors.New("invalid seat position")
	}

	level, err := p.getNumber("level", true)
	if err != nil {
		return nil, err
	}

	return map[vehicle.SeatPosition]vehicle.Level{seatPositions[int(index)]: vehicle.Level(level)}, nil
}

// Note: The API uses 0-3
func (p RequestParameters) settingForCoolerSeatPosition() (vehicle.Level, vehicle.SeatPosition, error) {
	position, err := p.getNumber("seat_position", true)
	if err != nil {
		return 0, 0, err
	}

	var seat vehicle.SeatPosition
	switch carserver.HvacSeatCoolerActions_HvacSeatCoolerPosition_E(position) {
	case carserver.HvacSeatCoolerActions_HvacSeatCoolerPosition_FrontLeft:
		seat = vehicle.SeatFrontLeft
	case carserver.HvacSeatCoolerActions_HvacSeatCoolerPosition_FrontRight:
		seat = vehicle.SeatFrontRight
	default:
		seat = vehicle.SeatUnknown
	}

	level, err := p.getNumberAny(true, "seat_cooler_level", "level")
	if err != nil {
		return 0, 0, err
	}

	return vehicle.Level(level - 1), seat, nil
}

func (p RequestParameters) settingForAutoSeatPosition() (vehicle.SeatPosition, bool, error) {
	position, err := p.getNumber("auto_seat_position", true)
	if err != nil {
		return 0, false, err
	}

	enabled, err := p.getBool("auto_climate_on", true)
	if err != nil {
		return 0, false, err
	}

	var seat vehicle.SeatPosition
	switch carserver.AutoSeatClimateAction_AutoSeatPosition_E(position) {
	case carserver.AutoSeatClimateAction_AutoSeatPosition_FrontLeft:
		seat = vehicle.SeatFrontLeft
	case carserver.AutoSeatClimateAction_AutoSeatPosition_FrontRight:
		seat = vehicle.SeatFrontRight
	default:
		seat = vehicle.SeatUnknown
	}

	return seat, enabled, nil
}

func (p RequestParameters) homelinkDevice() (vehicle.HomelinkDevice, error) {
	index, err := p.getOptionalUint32("homelink_device_index")
	if err != nil {
		return vehicle.HomelinkDevice{}, err
	}
	name, err := p.getString("homelink_device_name", false)
	if err != nil {
		return vehicle.HomelinkDevice{}, err
	}
	return vehicle.HomelinkDevice{Index: index, Name: name}, nil
}

func (p RequestParameters) suspensionLevel() (vehicle.SuspensionLevel, error) {
	if value, exists := p["suspension_level"]; exists {
		switch v := value.(type) {
		case string:
			level, err := vehicle.ParseSuspensionLevel(v)
			if err != nil {
				return 0, invalidParamError("suspension_level")
			}
			return level, nil
		case float64:
			if v != float64(int(v)) || v < 1 || v > 6 {
				return 0, invalidParamError("suspension_level")
			}
			return vehicle.SuspensionLevel(int(v)), nil
		default:
			return 0, invalidParamError("suspension_level")
		}
	}
	return 0, missingParamError("suspension_level")
}

func (p RequestParameters) getOptionalUint32(key string) (*uint32, error) {
	value, exists := p[key]
	if !exists {
		return nil, nil
	}
	num, isFloat64 := value.(float64)
	if !isFloat64 || num < 0 || num != float64(uint32(num)) {
		return nil, invalidParamError(key)
	}
	v := uint32(num)
	return &v, nil
}

func missingParamError(key string) error {
	return &protocol.NominalError{Details: fmt.Errorf("missing %s param", key)}
}

func invalidParamError(key string) error {
	return &protocol.NominalError{Details: fmt.Errorf("invalid %s param", key)}
}
