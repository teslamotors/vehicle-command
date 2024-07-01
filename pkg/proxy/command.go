package proxy

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/teslamotors/vehicle-command/pkg/connector/inet"
	"github.com/teslamotors/vehicle-command/pkg/protocol"
	"github.com/teslamotors/vehicle-command/pkg/vehicle"

	carserver "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/carserver"
)

var (
	// ErrCommandNotImplemented indicates a command has not be implemented in the SDK
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
		return nil, ErrCommandNotImplemented
	case "media_toggle_playback":
		return func(v *vehicle.Vehicle) error { return v.ToggleMediaPlayback(ctx) }, nil
	// Climate Controls
	case "auto_conditioning_start":
		return func(v *vehicle.Vehicle) error { return v.ClimateOn(ctx) }, nil
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
		driverTemp, err := params.getNumber("driver_temp", false)
		if err != nil {
			return nil, err
		}
		passengerTemp, err := params.getNumber("passenger_temp", false)
		if err != nil {
			return nil, err
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
	// Charging controls
	case "charge_standard":
		return func(v *vehicle.Vehicle) error { return v.ChargeStandardRange(ctx) }, nil
	case "charge_start":
		return func(v *vehicle.Vehicle) error { return v.ChargeStart(ctx) }, nil
	case "charge_stop":
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
	case "set_managed_charge_current_request":
		return nil, ErrCommandUseRESTAPI
	case "set_managed_charger_location":
		return nil, ErrCommandUseRESTAPI
	case "set_managed_scheduled_charging_time":
		return nil, ErrCommandUseRESTAPI
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
	case "wake_up":
		return func(v *vehicle.Vehicle) error { return v.Wakeup(ctx) }, nil
	// Security
	case "door_lock":
		return func(v *vehicle.Vehicle) error { return v.Lock(ctx) }, nil
	case "door_unlock":
		return func(v *vehicle.Vehicle) error { return v.Unlock(ctx) }, nil
	case "erase_user_data":
		return func(v *vehicle.Vehicle) error { return v.EraseGuestData(ctx) }, nil
	case "reset_pin_to_drive_pin":
		return func(v *vehicle.Vehicle) error { return v.ResetPIN(ctx) }, nil
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
		return func(v *vehicle.Vehicle) error { return v.SetValetMode(ctx, on, password) }, nil
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
	case "speed_limit_set_limit":
		speedMPH, err := params.getNumber("limit_mph", true)
		if err != nil {
			return nil, err
		}
		return func(v *vehicle.Vehicle) error { return v.SpeedLimitSetLimitMPH(ctx, speedMPH) }, nil
	case "trigger_homelink":
		lat, err := params.getNumber("lat", true)
		if err != nil {
			return nil, err
		}
		lon, err := params.getNumber("lon", true)
		if err != nil {
			return nil, err
		}
		return func(v *vehicle.Vehicle) error { return v.TriggerHomelink(ctx, float32(lat), float32(lon)) }, nil
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
		return nil, &inet.HttpError{Code: http.StatusBadRequest, Message: "{\"response\":null,\"error\":\"invalid_command\",\"error_description\":\"\"}"}
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
	index, err := p.getNumber("seat_position", true)
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

	level, err := p.getNumber("seat_cooler_level", true)
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

func missingParamError(key string) error {
	return &protocol.NominalError{Details: fmt.Errorf("missing %s param", key)}
}

func invalidParamError(key string) error {
	return &protocol.NominalError{Details: fmt.Errorf("invalid %s param", key)}
}
