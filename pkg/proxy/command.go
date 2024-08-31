package proxy

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/teslamotors/vehicle-command/pkg/action"
	"github.com/teslamotors/vehicle-command/pkg/protocol"
	carserver "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/carserver"
)

var (
	// ErrCommandNotImplemented indicates a command has not be implemented in the SDK
	ErrCommandNotImplemented = errors.New("command not implemented")

	// ErrCommandUseRESTAPI indicates vehicle/command is not supported by the protocol
	ErrCommandUseRESTAPI = errors.New("command requires using the REST API")

	seatPositions = []action.SeatPosition{
		action.SeatFrontLeft,
		action.SeatFrontRight,
		action.SeatSecondRowLeft,
		action.SeatSecondRowLeftBack,
		action.SeatSecondRowCenter,
		action.SeatSecondRowRight,
		action.SeatSecondRowRightBack,
		action.SeatThirdRowLeft,
		action.SeatThirdRowRight,
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
func ExtractCommandAction(command string, params RequestParameters) (interface{}, error) {
	switch command {
	// Media controls
	case "adjust_volume":
		volume, err := params.getNumber("volume", true)
		if err != nil {
			return nil, err
		}
		return action.SetVolume(float32(volume))
	case "remote_boombox":
		return nil, ErrCommandNotImplemented
	case "media_toggle_playback":
		return action.ToggleMediaPlayback(), nil
	// Climate Controls
	case "auto_conditioning_start":
		return action.ClimateOn(), nil
	case "auto_conditioning_stop":
		return action.ClimateOff(), nil
	case "charge_max_range":
		return action.ChargeMaxRange(), nil
	case "remote_seat_cooler_request":
		level, seat, err := params.settingForCoolerSeatPosition()
		if err != nil {
			return nil, err
		}
		return action.SetSeatCooler(level, seat)
	case "remote_seat_heater_request":
		setting, err := params.settingForHeatSeatPosition()
		if err != nil {
			return nil, err
		}

		return action.SetSeatHeater(setting), nil
	case "remote_auto_seat_climate_request":
		seat, enabled, err := params.settingForAutoSeatPosition()
		if err != nil {
			return nil, err
		}
		return action.AutoSeatAndClimate([]action.SeatPosition{seat}, enabled), nil
	case "remote_steering_wheel_heater_request":
		on, err := params.getBool("on", true)
		if err != nil {
			return nil, err
		}
		return action.SetSteeringWheelHeater(on), nil
	case "set_bioweapon_mode":
		on, err := params.getBool("on", true)
		if err != nil {
			return nil, err
		}
		override, err := params.getBool("manual_override", true)
		if err != nil {
			return nil, err
		}
		return action.SetBioweaponDefenseMode(on, override), nil
	case "set_cabin_overheat_protection":
		on, err := params.getBool("on", true)
		if err != nil {
			return nil, err
		}
		fanOnly, err := params.getBool("fan_only", false)
		if err != nil {
			return nil, err
		}
		return action.SetCabinOverheatProtection(on, fanOnly), nil
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
		return action.SetClimateKeeperMode(action.ClimateKeeperMode(mode), override), nil
	case "set_cop_temp":
		level, err := params.getNumber("cop_temp", true)
		if err != nil {
			return nil, err
		}
		return action.SetCabinOverheatProtectionTemperature(action.Level(level)), nil
	case "set_preconditioning_max":
		on, err := params.getBool("on", true)
		if err != nil {
			return nil, err
		}
		override, err := params.getBool("manual_override", false)
		if err != nil {
			return nil, err
		}
		return action.SetPreconditioningMax(on, override), nil
	case "set_temps":
		driverTemp, err := params.getNumber("driver_temp", false)
		if err != nil {
			return nil, err
		}
		passengerTemp, err := params.getNumber("passenger_temp", false)
		if err != nil {
			return nil, err
		}
		return action.ChangeClimateTemp(float32(driverTemp), float32(passengerTemp)), nil
	// vehicle.Vehicle actuation commands
	case "charge_port_door_open":
		return action.OpenChargePort(), nil
	case "charge_port_door_close":
		return action.CloseChargePort(), nil
	case "flash_lights":
		return action.FlashLights(), nil
	case "honk_horn":
		return action.HonkHorn(), nil
	case "actuate_trunk":
		if which, err := params.getString("which_trunk", false); err == nil {
			switch which {
			case "front":
				return action.OpenFrunk(), nil
			case "rear":
				return action.OpenTrunk(), nil
			default:
				return nil, &protocol.NominalError{Details: protocol.NewError("invalid_value", false, false)}
			}
		}
		return action.OpenTrunk(), nil
	case "open_tonneau":
		return action.OpenTonneau(), nil
	case "close_tonneau":
		return action.CloseTonneau(), nil
	case "stop_tonneau":
		return action.StopTonneau(), nil
	// Charging controls
	case "charge_standard":
		return action.ChargeStandardRange(), nil
	case "charge_start":
		return action.ChargeStart(), nil
	case "charge_stop":
		return action.ChargeStop(), nil
	case "set_charging_amps":
		amps, err := params.getNumber("charging_amps", true)
		if err != nil {
			return nil, err
		}
		return action.SetChargingAmps(int32(amps)), nil
	case "set_scheduled_charging":
		on, err := params.getBool("enable", true)
		if err != nil {
			return nil, err
		}
		scheduledTime, err := params.getTimeAfterMidnight("time")
		if err != nil {
			return nil, err
		}
		return action.ScheduleCharging(on, scheduledTime), nil
	case "set_charge_limit":
		limit, err := params.getNumber("percent", true)
		if err != nil {
			return nil, err
		}
		return action.ChangeChargeLimit(int32(limit)), nil
	case "set_scheduled_departure":
		enable, err := params.getBool("enable", true)
		if err != nil {
			return nil, err
		}
		if !enable {
			return action.ClearScheduledDeparture(), nil
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
		return action.ScheduleDeparture(departureTime, endOffPeakTime, preconditionPolicy, offPeakPolicy)
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
		schedule := carserver.ChargeSchedule{
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
		return action.AddChargeSchedule(&schedule), nil
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
		schedule := carserver.PreconditionSchedule{
			DaysOfWeek:       daysOfWeek,
			Latitude:         float32(lat),
			Longitude:        float32(lon),
			Id:               idUint64,
			PreconditionTime: int32(preconditionTime),
			OneTime:          oneTime,
			Enabled:          enabled,
		}
		return action.AddPreconditionSchedule(&schedule), nil
	case "remove_charge_schedule":
		id, err := params.getNumber("id", true)
		if err != nil {
			return nil, err
		}
		return action.RemoveChargeSchedule(uint64(id)), nil
	case "remove_precondition_schedule":
		id, err := params.getNumber("id", true)
		if err != nil {
			return nil, err
		}
		return action.RemovePreconditionSchedule(uint64(id)), nil
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
		return action.SetPINToDrive(on, password), nil
	// Security
	case "remote_start_drive":
		return action.RemoteDrive(), nil
	case "door_lock":
		return action.Lock(), nil
	case "door_unlock":
		return action.Unlock(), nil
	case "erase_user_data":
		return action.EraseGuestData(), nil
	case "reset_pin_to_drive_pin":
		return action.ResetPIN(), nil
	case "reset_valet_pin":
		return action.ResetValetPin(), nil
	case "guest_mode":
		on, err := params.getBool("enable", true)
		if err != nil {
			return nil, err
		}
		return action.SetGuestMode(on), nil
	case "set_sentry_mode":
		on, err := params.getBool("on", true)
		if err != nil {
			return nil, err
		}
		return action.SetSentryMode(on), nil
	case "set_valet_mode":
		on, err := params.getBool("on", true)
		if err != nil {
			return nil, err
		}
		password, err := params.getString("password", false)
		if err != nil {
			return nil, err
		}
		return action.SetValetMode(on, password), nil
	case "set_vehicle_name":
		name, err := params.getString("vehicle_name", true)
		if err != nil {
			return nil, err
		}
		return action.SetVehicleName(name), nil
	case "speed_limit_activate":
		pin, err := params.getString("pin", true)
		if err != nil {
			return nil, err
		}
		return action.ActivateSpeedLimit(pin), nil
	case "speed_limit_deactivate":
		pin, err := params.getString("pin", true)
		if err != nil {
			return nil, err
		}
		return action.DeactivateSpeedLimit(pin), nil
	case "speed_limit_clear_pin":
		pin, err := params.getString("pin", true)
		if err != nil {
			return nil, err
		}
		return action.ClearSpeedLimitPIN(pin), nil
	case "speed_limit_set_limit":
		speedMPH, err := params.getNumber("limit_mph", true)
		if err != nil {
			return nil, err
		}
		return action.SpeedLimitSetLimitMPH(speedMPH), nil
	case "trigger_homelink":
		lat, err := params.getNumber("lat", true)
		if err != nil {
			return nil, err
		}
		lon, err := params.getNumber("lon", true)
		if err != nil {
			return nil, err
		}
		return action.TriggerHomelink(float32(lat), float32(lon)), nil
	// Updates
	case "schedule_software_update":
		offsetSeconds, err := params.getNumber("offset_sec", true)
		if err != nil {
			return nil, err
		}
		return action.ScheduleSoftwareUpdate(time.Duration(offsetSeconds) * time.Second), nil
	case "cancel_software_update":
		return action.CancelSoftwareUpdate(), nil
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
			return action.VentWindows(), nil
		case "close":
			return action.CloseWindows(), nil
		default:
			return nil, errors.New("command must be 'vent' or 'close'")
		}
	default:
		return nil, nil
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

func (p RequestParameters) getPolicy(enabledKey string, weekdaysOnlyKey string) (action.ChargingPolicy, error) {
	enabled, err := p.getBool(enabledKey, false)
	if err != nil {
		return 0, err
	}
	weekdaysOnly, err := p.getBool(weekdaysOnlyKey, false)
	if err != nil {
		return 0, err
	}
	if weekdaysOnly {
		return action.ChargingPolicyWeekdays, nil
	}
	if enabled {
		return action.ChargingPolicyAllDays, nil
	}
	return action.ChargingPolicyOff, nil
}

func (p RequestParameters) getTimeAfterMidnight(key string) (time.Duration, error) {
	minutes, err := p.getNumber(key, false)
	if err != nil {
		return 0, err
	}
	// Leave further validation to the car for consistency with previous API.
	return time.Duration(minutes) * time.Minute, nil
}

func (p RequestParameters) settingForHeatSeatPosition() (map[action.SeatPosition]action.Level, error) {
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

	return map[action.SeatPosition]action.Level{seatPositions[int(index)]: action.Level(level)}, nil
}

// Note: The API uses 0-3
func (p RequestParameters) settingForCoolerSeatPosition() (action.Level, action.SeatPosition, error) {
	position, err := p.getNumber("seat_position", true)
	if err != nil {
		return 0, 0, err
	}

	var seat action.SeatPosition
	switch carserver.HvacSeatCoolerActions_HvacSeatCoolerPosition_E(position) {
	case carserver.HvacSeatCoolerActions_HvacSeatCoolerPosition_FrontLeft:
		seat = action.SeatFrontLeft
	case carserver.HvacSeatCoolerActions_HvacSeatCoolerPosition_FrontRight:
		seat = action.SeatFrontRight
	default:
		seat = action.SeatUnknown
	}

	level, err := p.getNumber("seat_cooler_level", true)
	if err != nil {
		return 0, 0, err
	}

	return action.Level(level - 1), seat, nil
}

func (p RequestParameters) settingForAutoSeatPosition() (action.SeatPosition, bool, error) {
	position, err := p.getNumber("auto_seat_position", true)
	if err != nil {
		return 0, false, err
	}

	enabled, err := p.getBool("auto_climate_on", true)
	if err != nil {
		return 0, false, err
	}

	var seat action.SeatPosition
	switch carserver.AutoSeatClimateAction_AutoSeatPosition_E(position) {
	case carserver.AutoSeatClimateAction_AutoSeatPosition_FrontLeft:
		seat = action.SeatFrontLeft
	case carserver.AutoSeatClimateAction_AutoSeatPosition_FrontRight:
		seat = action.SeatFrontRight
	default:
		seat = action.SeatUnknown
	}

	return seat, enabled, nil
}

func missingParamError(key string) error {
	return &protocol.NominalError{Details: fmt.Errorf("missing %s param", key)}
}

func invalidParamError(key string) error {
	return &protocol.NominalError{Details: fmt.Errorf("invalid %s param", key)}
}
