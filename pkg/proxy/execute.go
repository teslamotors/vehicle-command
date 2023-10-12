package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/teslamotors/vehicle-command/pkg/connector/inet"
	"github.com/teslamotors/vehicle-command/pkg/protocol"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/carserver"
	"github.com/teslamotors/vehicle-command/pkg/vehicle"
)

var (
	ErrNotImplemented = errors.New("command not implemented")
	ErrUseRESTAPI     = errors.New("command requires using the REST API")
)

func missingParamError(key string) error {
	return &protocol.NominalError{Details: fmt.Errorf("missing %s param", key)}
}

func invalidParamError(key string) error {
	return &protocol.NominalError{Details: fmt.Errorf("invalid %s param", key)}
}

type requestParameters map[string]interface{}

func (p requestParameters) getString(key string, required bool) (string, error) {
	if value, ok := p[key]; ok {
		if s, ok := value.(string); ok {
			return s, nil
		} else {
			return "", invalidParamError(key)
		}
	} else if !required {
		return "", nil
	}
	return "", missingParamError(key)
}

func (p requestParameters) getBool(key string, required bool) (bool, error) {
	if value, ok := p[key]; ok {
		if s, ok := value.(bool); ok {
			return s, nil
		} else {
			return false, invalidParamError(key)
		}
	} else if !required {
		return false, nil
	}
	return false, missingParamError(key)
}

func (p requestParameters) getNumber(key string, required bool) (float64, error) {
	if value, ok := p[key]; ok {
		if s, ok := value.(float64); ok {
			return s, nil
		} else {
			return 0, invalidParamError(key)
		}
	} else if !required {
		return 0, nil
	}
	return 0, missingParamError(key)
}

func (p requestParameters) getPolicy(enabledKey string, weekdaysOnlyKey string) (vehicle.ChargingPolicy, error) {
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

func (p requestParameters) getTimeAfterMidnight(key string) (time.Duration, error) {
	minutes, err := p.getNumber(key, false)
	if err != nil {
		return 0, err
	}
	// Leave further validation to the car for consistency with previous API.
	return time.Duration(minutes) * time.Minute, nil
}

func execute(ctx context.Context, req *http.Request, car *vehicle.Vehicle, command string) error {
	var params requestParameters
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return err
	}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &params); err != nil {
			return &inet.HttpError{Code: http.StatusBadRequest, Message: "invalid JSON: Error occurred while parsing request parameters"}
		}
	}
	switch command {
	// Media controls
	case "adjust_volume":
		volume, err := params.getNumber("volume", true)
		if err != nil {
			return err
		}
		return car.SetVolume(ctx, float32(volume))
	case "remote_boombox":
		return ErrNotImplemented
	// Climate Controls
	case "auto_conditioning_start":
		return car.ClimateOn(ctx)
	case "auto_conditioning_stop":
		return car.ClimateOff(ctx)
	case "charge_max_range":
		return car.ChargeMaxRange(ctx)
	case "remote_seat_cooler_request":
		position, err := params.getNumber("seat_position", true)
		if err != nil {
			return err
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
		level, err := params.getNumber("seat_cooler_level", true)
		if err != nil {
			return err
		}
		// Our API uses 0-3
		return car.SetSeatCooler(ctx, vehicle.Level(level-1), seat)
	case "remote_seat_heater_request":
		positions := []vehicle.SeatPosition{
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
		index, err := params.getNumber("seat_position", true)
		if err != nil {
			return err
		}
		if int(index) < 0 || int(index) >= len(positions) {
			return errors.New("invalid seat position")
		}

		level, err := params.getNumber("level", true)
		if err != nil {
			return err
		}

		setting := map[vehicle.SeatPosition]vehicle.Level{
			positions[int(index)]: vehicle.Level(level),
		}

		return car.SetSeatHeater(ctx, setting)
	case "remote_auto_seat_climate_request":
		position, err := params.getNumber("auto_seat_position", true)
		if err != nil {
			return err
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

		level, err := params.getNumber("seat_position", true)
		if err != nil {
			return err
		}

		return car.SetSeatCooler(ctx, vehicle.Level(level-1), seat)
	case "remote_steering_wheel_heater_request":
		on, err := params.getBool("on", true)
		if err != nil {
			return err
		}
		return car.SetSteeringWheelHeater(ctx, on)
	case "set_bioweapon_mode":
		on, err := params.getBool("on", true)
		if err != nil {
			return err
		}
		override, err := params.getBool("manual_override", true)
		if err != nil {
			return err
		}
		return car.SetBioweaponDefenseMode(ctx, on, override)
	case "set_cabin_overheat_protection":
		on, err := params.getBool("on", true)
		if err != nil {
			return err
		}
		fanOnly, err := params.getBool("fan_only", false)
		if err != nil {
			return err
		}
		return car.SetCabinOverheatProtection(ctx, on, fanOnly)
	case "set_climate_keeper_mode":
		// 0 : off
		// 1 : On
		// 2 : Dog
		// 3 : Camp
		mode, err := params.getNumber("climate_keeper_mode", true)
		if err != nil {
			return err
		}
		override, err := params.getBool("manual_override", false)
		if err != nil {
			return err
		}
		return car.SetClimateKeeperMode(ctx, vehicle.ClimateKeeperMode(mode), override)
	case "set_cop_temp":
		level, err := params.getNumber("cop_temp", true)
		if err != nil {
			return nil
		}
		return car.SetCabinOverheatProtectionTemperature(ctx, vehicle.Level(level))
	case "set_preconditioning_max":
		on, err := params.getBool("on", true)
		if err != nil {
			return err
		}
		override, err := params.getBool("manual_override", false)
		if err != nil {
			return err
		}
		return car.SetPreconditioningMax(ctx, on, override)
	case "set_temps":
		driverTemp, err := params.getNumber("driver_temp", false)
		if err != nil {
			return err
		}
		passengerTemp, err := params.getNumber("passenger_temp", false)
		if err != nil {
			return err
		}
		return car.ChangeClimateTemp(ctx, float32(driverTemp), float32(passengerTemp))
	// Vehicle actuation commands
	case "actuate_trunk":
		if which, err := params.getString("which_trunk", false); err == nil {
			switch which {
			case "front":
				return car.OpenFrunk(ctx)
			case "rear":
				return car.OpenTrunk(ctx)
			default:
				return &protocol.NominalError{
					Details: protocol.NewError("invalid_value", false, false),
				}
			}
		}
		return car.OpenTrunk(ctx)
	case "charge_port_door_open":
		return car.ChargePortOpen(ctx)
	case "charge_port_door_close":
		return car.ChargePortClose(ctx)
	case "flash_lights":
		return car.FlashLights(ctx)
	case "honk_horn":
		return car.HonkHorn(ctx)
	case "remote_start_drive":
		return car.RemoteDrive(ctx)
	// Charging controls
	case "charge_standard":
		return car.ChargeStandardRange(ctx)
	case "charge_start":
		return car.ChargeStart(ctx)
	case "charge_stop":
		return car.ChargeStop(ctx)
	case "set_charging_amps":
		amps, err := params.getNumber("charging_amps", true)
		if err != nil {
			return err
		}
		return car.SetChargingAmps(ctx, int32(amps))
	case "set_scheduled_charging":
		on, err := params.getBool("enable", true)
		if err != nil {
			return err
		}
		scheduledTime, err := params.getTimeAfterMidnight("time")
		if err != nil {
			return err
		}
		return car.ScheduleCharging(ctx, on, scheduledTime)
	case "set_charge_limit":
		limit, err := params.getNumber("percent", true)
		if err != nil {
			return err
		}
		return car.ChangeChargeLimit(ctx, int32(limit))
	case "set_scheduled_departure":
		enable, err := params.getBool("enable", true)
		if err != nil {
			return err
		}
		if !enable {
			return car.ClearScheduledDeparture(ctx)
		}

		offPeakPolicy, err := params.getPolicy("off_peak_charging_enabled", "off_peak_charging_weekdays_only")
		if err != nil {
			return err
		}
		preconditionPolicy, err := params.getPolicy("preconditioning_enabled", "preconditioning_weekdays_only")
		if err != nil {
			return nil
		}

		departureTime, err := params.getTimeAfterMidnight("departure_time")
		if err != nil {
			return err
		}
		endOffPeakTime, err := params.getTimeAfterMidnight("end_off_peak_time")
		if err != nil {
			return err
		}
		return car.ScheduleDeparture(ctx, departureTime, endOffPeakTime, preconditionPolicy, offPeakPolicy)
	case "set_managed_charge_current_request":
		return ErrUseRESTAPI
	case "set_managed_charger_location":
		return ErrUseRESTAPI
	case "set_managed_scheduled_charging_time":
		return ErrUseRESTAPI
	case "set_pin_to_drive":
		on, err := params.getBool("on", true)
		if err != nil {
			return err
		}
		password, err := params.getString("password", false)
		if err != nil {
			return err
		}
		return car.SetPINToDrive(ctx, on, password)
	case "wake_up":
		return car.Wakeup(ctx)
	// Security
	case "door_lock":
		return car.Lock(ctx)
	case "door_unlock":
		return car.Unlock(ctx)
	case "reset_pin_to_drive_pin":
		return car.ResetPIN(ctx)
	case "reset_valet_pin":
		return car.ResetValetPin(ctx)
	case "guest_mode":
		on, err := params.getBool("enable", true)
		if err != nil {
			return err
		}
		return car.SetGuestMode(ctx, on)
	case "set_sentry_mode":
		on, err := params.getBool("on", true)
		if err != nil {
			return err
		}
		return car.SetSentryMode(ctx, on)
	case "set_valet_mode":
		on, err := params.getBool("on", true)
		if err != nil {
			return err
		}
		password, err := params.getString("password", false)
		if err != nil {
			return err
		}
		return car.SetValetMode(ctx, on, password)
	case "set_vehicle_name":
		name, err := params.getString("vehicle_name", true)
		if err != nil {
			return err
		}
		return car.SetVehicleName(ctx, name)
	case "speed_limit_activate":
		pin, err := params.getString("pin", true)
		if err != nil {
			return err
		}
		return car.ActivateSpeedLimit(ctx, pin)
	case "speed_limit_deactivate":
		pin, err := params.getString("pin", true)
		if err != nil {
			return err
		}
		return car.DeactivateSpeedLimit(ctx, pin)
	case "speed_limit_clear_pin":
		pin, err := params.getString("pin", true)
		if err != nil {
			return err
		}
		return car.ClearSpeedLimitPIN(ctx, pin)
	case "speed_limit_set_limit":
		speedMPH, err := params.getNumber("limit_mph", true)
		if err != nil {
			return err
		}
		return car.SpeedLimitSetLimitMPH(ctx, speedMPH)
	case "trigger_homelink":
		lat, err := params.getNumber("lat", true)
		if err != nil {
			return err
		}
		lon, err := params.getNumber("lon", true)
		if err != nil {
			return err
		}
		return car.TriggerHomelink(ctx, float32(lat), float32(lon))
	// Updates
	case "schedule_software_update":
		offsetSeconds, err := params.getNumber("offset_sec", true)
		if err != nil {
			return err
		}
		return car.ScheduleSoftwareUpdate(ctx, time.Duration(offsetSeconds)*time.Second)
	case "cancel_software_update":
		return car.CancelSoftwareUpdate(ctx)
	// Sharing options. These endpoints often require server-side processing, which prevents strict
	// end-to-end authentication.
	case "navigation_request":
		return ErrUseRESTAPI
	default:
		return &inet.HttpError{Code: http.StatusBadRequest, Message: "{\"response\":null,\"error\":\"invalid_command\",\"error_description\":\"\"}"}
	}
}
