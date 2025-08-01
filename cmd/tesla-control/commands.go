package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/teslamotors/vehicle-command/pkg/account"
	"github.com/teslamotors/vehicle-command/pkg/cli"
	"github.com/teslamotors/vehicle-command/pkg/protocol"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/keys"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/vcsec"
	"github.com/teslamotors/vehicle-command/pkg/vehicle"
	"google.golang.org/protobuf/encoding/protojson"
)

var (
	ErrCommandLineArgs = errors.New("invalid command line arguments")
	ErrInvalidTime     = errors.New("invalid time")
	dayNamesBitMask    = map[string]int32{
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

type Argument struct {
	name string
	help string
}

type Handler func(ctx context.Context, acct *account.Account, car *vehicle.Vehicle, args map[string]string) error

type Command struct {
	help             string
	requiresAuth     bool // True if command requires client-to-vehicle authentication (private key)
	requiresFleetAPI bool // True if command requires client-to-server authentication (OAuth token)
	args             []Argument
	optional         []Argument
	handler          Handler
	domain           protocol.Domain
}

var categoriesByName = map[string]vehicle.StateCategory{
	"charge":                vehicle.StateCategoryCharge,
	"climate":               vehicle.StateCategoryClimate,
	"drive":                 vehicle.StateCategoryDrive,
	"location":              vehicle.StateCategoryLocation,
	"closures":              vehicle.StateCategoryClosures,
	"charge-schedule":       vehicle.StateCategoryChargeSchedule,
	"precondition-schedule": vehicle.StateCategoryPreconditioningSchedule,
	"tire-pressure":         vehicle.StateCategoryTirePressure,
	"media":                 vehicle.StateCategoryMedia,
	"media-detail":          vehicle.StateCategoryMediaDetail,
	"software-update":       vehicle.StateCategorySoftwareUpdate,
	"parental-controls":     vehicle.StateCategoryParentalControls,
}

func categoryNames() []string {
	var names []string
	for name := range categoriesByName {
		names = append(names, name)
	}
	return names
}

func GetCategory(nameStr string) (vehicle.StateCategory, error) {
	if category, ok := categoriesByName[strings.ToLower(nameStr)]; ok {
		return category, nil
	}
	return 0, fmt.Errorf("unrecognized state category '%s'", nameStr)
}

func GetDegree(degStr string) (float32, error) {
	deg, err := strconv.ParseFloat(degStr, 32)
	if err != nil {
		return 0.0, err
	}
	if deg < -180 || deg > 180 {
		return 0.0, errors.New("latitude and longitude must both be in the range [-180, 180]")
	}
	return float32(deg), nil
}

func GetDays(days string) (int32, error) {
	var mask int32
	for _, d := range strings.Split(days, ",") {
		if v, ok := dayNamesBitMask[strings.TrimSpace(strings.ToUpper(d))]; ok {
			mask |= v
		} else {
			return 0, fmt.Errorf("unrecognized day name: %v", d)
		}
	}
	return mask, nil
}

func MinutesAfterMidnight(hoursAndMinutes string) (int32, error) {
	components := strings.Split(hoursAndMinutes, ":")
	if len(components) != 2 {
		return 0, fmt.Errorf("%w: expected HH:MM", ErrInvalidTime)
	}
	hours, err := strconv.Atoi(components[0])
	if err != nil {
		return 0, fmt.Errorf("%w: %s", ErrInvalidTime, err)
	}
	minutes, err := strconv.Atoi(components[1])
	if err != nil {
		return 0, fmt.Errorf("%w: %s", ErrInvalidTime, err)
	}

	if hours > 23 || hours < 0 || minutes > 59 || minutes < 0 {
		return 0, fmt.Errorf("%w: hours or minutes outside valid range", ErrInvalidTime)
	}
	return int32(60*hours + minutes), nil
}

// configureAndVerifyFlags verifies that c contains all the information required to execute a command.
func configureFlags(c *cli.Config, commandName string, forceBLE bool) error {
	info, ok := commands[commandName]
	if !ok {
		return ErrUnknownCommand
	}
	c.Flags = cli.FlagBLE
	if info.domain != protocol.DomainNone {
		c.Domains = cli.DomainList{info.domain}
	}
	bleWake := forceBLE && commandName == "wake"
	if bleWake || info.requiresAuth {
		// Wake commands are special. When sending a wake command over the Internet, infotainment
		// cannot authenticate the command because it's asleep. When sending the command over BLE,
		// VCSEC _does_ authenticate the command before poking infotainment.
		c.Flags |= cli.FlagPrivateKey | cli.FlagVIN
	}
	if bleWake {
		// Normally, clients send out two handshake messages in parallel in order to reduce latency.
		// One handshake with VCSEC, one handshake with infotainment. However, if we're sending a
		// BLE wake command, then infotainment is (presumably) asleep, and so we should only try to
		// handshake with VCSEC.
		c.Domains = cli.DomainList{protocol.DomainVCSEC}
	}
	if !info.requiresFleetAPI {
		c.Flags |= cli.FlagVIN
	}
	if forceBLE {
		if info.requiresFleetAPI {
			return ErrRequiresOAuth
		}
	} else {
		c.Flags |= cli.FlagOAuth
	}

	// Verify all required parameters are present.
	havePrivateKey := !(c.KeyringKeyName == "" && c.KeyFilename == "")
	haveOAuth := !(c.KeyringTokenName == "" && c.TokenFilename == "")
	haveVIN := c.VIN != ""
	_, err := checkReadiness(commandName, havePrivateKey, haveOAuth, haveVIN)
	return err
}

var (
	ErrRequiresOAuth      = errors.New("command requires a FleetAPI OAuth token")
	ErrRequiresVIN        = errors.New("command requires a VIN")
	ErrRequiresPrivateKey = errors.New("command requires a private key")
	ErrUnknownCommand     = errors.New("unrecognized command")
)

func checkReadiness(commandName string, havePrivateKey, haveOAuth, haveVIN bool) (*Command, error) {
	info, ok := commands[commandName]
	if !ok {
		return nil, ErrUnknownCommand
	}
	if info.requiresFleetAPI {
		if !haveOAuth {
			return nil, ErrRequiresOAuth
		}
	} else {
		// Currently, commands supported by this application either target the account (and
		// therefore require FleetAPI credentials but not a VIN) or target a vehicle (and therefore
		// require a VIN but not FleetAPI credentials).
		if !haveVIN {
			return nil, ErrRequiresVIN
		}
	}
	if info.requiresAuth && !havePrivateKey {
		return nil, ErrRequiresPrivateKey
	}
	return info, nil
}

func execute(ctx context.Context, acct *account.Account, car *vehicle.Vehicle, args []string) error {
	if len(args) == 0 {
		return errors.New("missing COMMAND")
	}

	info, err := checkReadiness(args[0], car != nil && car.PrivateKeyAvailable(), acct != nil, car != nil)
	if err != nil {
		return err
	}

	if len(args)-1 < len(info.args) || len(args)-1 > len(info.args)+len(info.optional) {
		writeErr("Invalid number of command line arguments: %d (%d required, %d optional).", len(args), len(info.args), len(info.optional))
		err = ErrCommandLineArgs
	} else {
		keywords := make(map[string]string)
		for i, argInfo := range info.args {
			keywords[argInfo.name] = args[i+1]
		}
		index := len(info.args) + 1
		for _, argInfo := range info.optional {
			if index >= len(args) {
				break
			}
			keywords[argInfo.name] = args[index]
			index++
		}
		err = info.handler(ctx, acct, car, keywords)
	}

	// Print command-specific help
	if errors.Is(err, ErrCommandLineArgs) {
		info.Usage(args[0])
	}
	return err
}

func (c *Command) Usage(name string) {
	fmt.Printf("Usage: %s", name)
	maxLength := 0
	for _, arg := range c.args {
		fmt.Printf(" %s", arg.name)
		if len(arg.name) > maxLength {
			maxLength = len(arg.name)
		}
	}
	if len(c.optional) > 0 {
		fmt.Printf(" [")
	}
	for _, arg := range c.optional {
		fmt.Printf(" %s", arg.name)
		if len(arg.name) > maxLength {
			maxLength = len(arg.name)
		}
	}
	if len(c.optional) > 0 {
		fmt.Printf(" ]")
	}
	fmt.Printf("\n%s\n", c.help)
	maxLength++
	for _, arg := range c.args {
		fmt.Printf("    %s:%s%s\n", arg.name, strings.Repeat(" ", maxLength-len(arg.name)), arg.help)
	}
	for _, arg := range c.optional {
		fmt.Printf("    %s:%s%s\n", arg.name, strings.Repeat(" ", maxLength-len(arg.name)), arg.help)
	}
}

var commands = map[string]*Command{
	"valet-mode-on": {
		help:             "Enable valet mode",
		requiresAuth:     true,
		requiresFleetAPI: false,
		args: []Argument{
			{name: "PIN", help: "Valet mode PIN"},
		},
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, args map[string]string) error {
			return car.EnableValetMode(ctx, args["PIN"])
		},
	},
	"valet-mode-off": {
		help:             "Disable valet mode",
		requiresAuth:     true,
		requiresFleetAPI: false,
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, _ map[string]string) error {
			return car.DisableValetMode(ctx)
		},
	},
	"unlock": {
		help:             "Unlock vehicle",
		requiresAuth:     true,
		requiresFleetAPI: false,
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, _ map[string]string) error {
			return car.Unlock(ctx)
		},
	},
	"lock": {
		help:             "Lock vehicle",
		requiresAuth:     true,
		requiresFleetAPI: false,
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, _ map[string]string) error {
			return car.Lock(ctx)
		},
	},
	"drive": {
		help:             "Remote start vehicle",
		requiresAuth:     true,
		requiresFleetAPI: false,
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, _ map[string]string) error {
			return car.RemoteDrive(ctx)
		},
	},
	"climate-on": {
		help:             "Turn on climate control",
		requiresAuth:     true,
		requiresFleetAPI: false,
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, _ map[string]string) error {
			return car.ClimateOn(ctx)
		},
	},
	"climate-off": {
		help:             "Turn off climate control",
		requiresAuth:     true,
		requiresFleetAPI: false,
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, _ map[string]string) error {
			return car.ClimateOff(ctx)
		},
	},
	"climate-set-temp": {
		help:             "Set temperature (Celsius)",
		requiresAuth:     true,
		requiresFleetAPI: false,
		args: []Argument{
			{name: "TEMP", help: "Desired temperature (e.g., 70f or 21c; defaults to Celsius)"},
		},
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, args map[string]string) error {
			var degrees float32
			var unit string
			if _, err := fmt.Sscanf(args["TEMP"], "%f%s", &degrees, &unit); err != nil {
				return fmt.Errorf("failed to parse temperature: format as 22C or 72F")
			}
			if unit == "F" || unit == "f" {
				degrees = (degrees - 32.0) * 5.0 / 9.0
			} else if unit != "C" && unit != "c" {
				return fmt.Errorf("temperature units must be C or F")
			}
			return car.ChangeClimateTemp(ctx, degrees, degrees)
		},
	},
	"add-key": {
		help:             "Add PUBLIC_KEY to vehicle whitelist with ROLE and FORM_FACTOR",
		requiresAuth:     true,
		requiresFleetAPI: false,
		args: []Argument{
			{name: "PUBLIC_KEY", help: "file containing public key (or corresponding private key)"},
			{name: "ROLE", help: "One of: owner, driver, fm (fleet manager), vehicle_monitor, charging_manager"},
			{name: "FORM_FACTOR", help: "One of: nfc_card, ios_device, android_device, cloud_key"},
		},
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, args map[string]string) error {
			role, ok := keys.Role_value["ROLE_"+strings.ToUpper(args["ROLE"])]
			if !ok {
				return fmt.Errorf("%w: invalid ROLE", ErrCommandLineArgs)
			}
			formFactor, ok := vcsec.KeyFormFactor_value["KEY_FORM_FACTOR_"+strings.ToUpper(args["FORM_FACTOR"])]
			if !ok {
				return fmt.Errorf("%w: unrecognized FORM_FACTOR", ErrCommandLineArgs)
			}
			publicKey, err := protocol.LoadPublicKey(args["PUBLIC_KEY"])
			if err != nil {
				return fmt.Errorf("invalid public key: %s", err)
			}
			return car.AddKeyWithRole(ctx, publicKey, keys.Role(role), vcsec.KeyFormFactor(formFactor))
		},
	},
	"add-key-request": {
		help:             "Request NFC-card approval for a enrolling PUBLIC_KEY with ROLE and FORM_FACTOR",
		requiresAuth:     false,
		requiresFleetAPI: false,
		args: []Argument{
			{name: "PUBLIC_KEY", help: "file containing public key (or corresponding private key)"},
			{name: "ROLE", help: "One of: owner, driver, fm (fleet manager), vehicle_monitor, charging_manager"},
			{name: "FORM_FACTOR", help: "One of: nfc_card, ios_device, android_device, cloud_key"},
		},
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, args map[string]string) error {
			role, ok := keys.Role_value["ROLE_"+strings.ToUpper(args["ROLE"])]
			if !ok {
				return fmt.Errorf("%w: invalid ROLE", ErrCommandLineArgs)
			}
			formFactor, ok := vcsec.KeyFormFactor_value["KEY_FORM_FACTOR_"+strings.ToUpper(args["FORM_FACTOR"])]
			if !ok {
				return fmt.Errorf("%w: unrecognized FORM_FACTOR", ErrCommandLineArgs)
			}
			publicKey, err := protocol.LoadPublicKey(args["PUBLIC_KEY"])
			if err != nil {
				return fmt.Errorf("invalid public key: %s", err)
			}
			if err := car.SendAddKeyRequestWithRole(ctx, publicKey, keys.Role(role), vcsec.KeyFormFactor(formFactor)); err != nil {
				return err
			}
			fmt.Printf("Sent add-key request to %s. Confirm by tapping NFC card on center console.\n", car.VIN())
			return nil
		},
	},
	"remove-key": {
		help:             "Remove PUBLIC_KEY from vehicle whitelist",
		requiresAuth:     true,
		requiresFleetAPI: false,
		args: []Argument{
			{name: "PUBLIC_KEY", help: "file containing public key (or corresponding private key)"},
		},
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, args map[string]string) error {
			publicKey, err := protocol.LoadPublicKey(args["PUBLIC_KEY"])
			if err != nil {
				return fmt.Errorf("invalid public key: %s", err)
			}
			return car.RemoveKey(ctx, publicKey)
		},
	},
	"rename-key": {
		help:             "Change the human-readable metadata of PUBLIC_KEY to NAME, MODEL, KIND",
		requiresAuth:     false,
		requiresFleetAPI: true,
		args: []Argument{
			{name: "PUBLIC_KEY", help: "file containing public key (or corresponding private key)"},
			{name: "NAME", help: "New human-readable name for the public key (e.g., Dave's Phone)"},
		},
		handler: func(ctx context.Context, acct *account.Account, _ *vehicle.Vehicle, args map[string]string) error {
			publicKey, err := protocol.LoadPublicKey(args["PUBLIC_KEY"])
			if err != nil {
				return fmt.Errorf("invalid public key: %s", err)
			}
			return acct.UpdateKey(ctx, publicKey, args["NAME"])
		},
	},
	"get": {
		help:             "GET an owner API http ENDPOINT. Hostname will be taken from -config.",
		requiresAuth:     false,
		requiresFleetAPI: true,
		args: []Argument{
			{name: "ENDPOINT", help: "Fleet API endpoint"},
		},
		handler: func(ctx context.Context, acct *account.Account, _ *vehicle.Vehicle, args map[string]string) error {
			reply, err := acct.Get(ctx, args["ENDPOINT"])
			if err != nil {
				return err
			}
			fmt.Println(string(reply))
			return nil
		},
	},
	"post": {
		help:             "POST to ENDPOINT the contents of FILE. Hostname will be taken from -config.",
		requiresAuth:     false,
		requiresFleetAPI: true,
		args: []Argument{
			{name: "ENDPOINT", help: "Fleet API endpoint"},
		},
		optional: []Argument{
			{name: "FILE", help: "JSON file to POST"},
		},
		handler: func(ctx context.Context, acct *account.Account, _ *vehicle.Vehicle, args map[string]string) error {
			var jsonBytes []byte
			var err error
			if filename, ok := args["FILE"]; ok {
				jsonBytes, err = os.ReadFile(filename)
			} else {
				jsonBytes, err = io.ReadAll(os.Stdin)
			}
			if err != nil {
				return err
			}
			reply, err := acct.Post(ctx, args["ENDPOINT"], jsonBytes)
			// reply can be set where there's an error; typically a JSON blob providing details
			if reply != nil {
				fmt.Println(string(reply))
			}
			if err != nil {
				return err
			}
			return nil
		},
	},
	"list-keys": {
		help:             "List public keys enrolled on vehicle",
		requiresAuth:     false,
		requiresFleetAPI: false,
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, _ map[string]string) error {
			summary, err := car.KeySummary(ctx)
			if err != nil {
				return err
			}
			slot := uint32(0)
			var details *vcsec.WhitelistEntryInfo
			for mask := summary.GetSlotMask(); mask > 0; mask >>= 1 {
				if mask&1 == 1 {
					details, err = car.KeyInfoBySlot(ctx, slot)
					if err != nil {
						writeErr("Error fetching slot %d: %s", slot, err)
						if errors.Is(err, context.DeadlineExceeded) {
							return err
						}
					}
					if details != nil {
						fmt.Printf("%02x\t%s\t%s\n", details.GetPublicKey().GetPublicKeyRaw(), details.GetKeyRole(), details.GetMetadataForKey().GetKeyFormFactor())
					}
				}
				slot++
			}
			return nil
		},
	},
	"honk": {
		help:             "Honk horn",
		requiresAuth:     true,
		requiresFleetAPI: false,
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, _ map[string]string) error {
			return car.HonkHorn(ctx)
		},
	},
	"ping": {
		help:             "Ping vehicle",
		requiresAuth:     true,
		requiresFleetAPI: false,
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, _ map[string]string) error {
			return car.Ping(ctx)
		},
	},
	"flash-lights": {
		help:             "Flash lights",
		requiresAuth:     true,
		requiresFleetAPI: false,
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, _ map[string]string) error {
			return car.FlashLights(ctx)
		},
	},
	"charging-set-limit": {
		help:             "Set charge limit to PERCENT",
		requiresAuth:     true,
		requiresFleetAPI: false,
		args: []Argument{
			{name: "PERCENT", help: "Charging limit"},
		},
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, args map[string]string) error {
			limit, err := strconv.Atoi(args["PERCENT"])
			if err != nil {
				return fmt.Errorf("error parsing PERCENT")
			}
			return car.ChangeChargeLimit(ctx, int32(limit))
		},
	},
	"charging-set-amps": {
		help:             "Set charge current to AMPS",
		requiresAuth:     true,
		requiresFleetAPI: false,
		args: []Argument{
			{name: "AMPS", help: "Charging current"},
		},
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, args map[string]string) error {
			limit, err := strconv.Atoi(args["AMPS"])
			if err != nil {
				return fmt.Errorf("error parsing AMPS")
			}
			return car.SetChargingAmps(ctx, int32(limit))
		},
	},
	"charging-start": {
		help:             "Start charging",
		requiresAuth:     true,
		requiresFleetAPI: false,
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, _ map[string]string) error {
			return car.ChargeStart(ctx)
		},
	},
	"charging-stop": {
		help:             "Stop charging",
		requiresAuth:     true,
		requiresFleetAPI: false,
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, _ map[string]string) error {
			return car.ChargeStop(ctx)
		},
	},
	"charging-schedule": {
		help:             "Schedule charging to MINS minutes after midnight and enable daily scheduling",
		requiresAuth:     true,
		requiresFleetAPI: false,
		args: []Argument{
			{name: "MINS", help: "Time after midnight in minutes"},
		},
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, args map[string]string) error {
			minutesAfterMidnight, err := strconv.Atoi(args["MINS"])
			if err != nil {
				return fmt.Errorf("error parsing minutes")
			}
			// Convert minutes to a time.Duration
			chargingTime := time.Duration(minutesAfterMidnight) * time.Minute
			return car.ScheduleCharging(ctx, true, chargingTime)
		},
	},
	"charging-schedule-cancel": {
		help:             "Cancel scheduled charge start",
		requiresAuth:     true,
		requiresFleetAPI: false,
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, _ map[string]string) error {
			return car.ScheduleCharging(ctx, false, 0*time.Hour)
		},
	},
	"media-set-volume": {
		help:             "Set volume",
		requiresAuth:     true,
		requiresFleetAPI: false,
		args: []Argument{
			{name: "VOLUME", help: "Set volume (0.0-10.0"},
		},
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, args map[string]string) error {
			volume, err := strconv.ParseFloat(args["VOLUME"], 32)
			if err != nil {
				return fmt.Errorf("failed to parse volume")
			}
			return car.SetVolume(ctx, float32(volume))
		},
	},
	"media-volume-up": {
		help:             "Increase volume",
		requiresAuth:     true,
		requiresFleetAPI: false,
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, _ map[string]string) error {
			return car.VolumeUp(ctx)
		},
	},
	"media-volume-down": {
		help:             "Decrease volume",
		requiresAuth:     true,
		requiresFleetAPI: false,
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, _ map[string]string) error {
			return car.VolumeDown(ctx)
		},
	},
	"media-next-favorite": {
		help:             "Next favorite",
		requiresAuth:     true,
		requiresFleetAPI: false,
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, _ map[string]string) error {
			return car.MediaNextFavorite(ctx)
		},
	},
	"media-next-track": {
		help:             "Next track",
		requiresAuth:     true,
		requiresFleetAPI: false,
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, _ map[string]string) error {
			return car.MediaNextTrack(ctx)
		},
	},
	"media-previous-track": {
		help:             "Previous track",
		requiresAuth:     true,
		requiresFleetAPI: false,
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, _ map[string]string) error {
			return car.MediaPreviousTrack(ctx)
		},
	},

	"media-previous-favorite": {
		help:             "Previous favorite",
		requiresAuth:     true,
		requiresFleetAPI: false,
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, _ map[string]string) error {
			return car.MediaPreviousFavorite(ctx)
		},
	},
	"media-toggle-playback": {
		help:             "Toggle between play/pause",
		requiresAuth:     true,
		requiresFleetAPI: false,
		args:             []Argument{},
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, _ map[string]string) error {
			return car.ToggleMediaPlayback(ctx)
		},
	},
	"software-update-start": {
		help:             "Start software update after DELAY",
		requiresAuth:     true,
		requiresFleetAPI: false,
		args: []Argument{
			{
				name: "DELAY",
				help: "Time to wait before starting update. Examples: 2h, 10m.",
			},
		},
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, args map[string]string) error {
			delay, err := time.ParseDuration(args["DELAY"])
			if err != nil {
				return fmt.Errorf("error parsing DELAY. Valid times are <n><unit>, where <n> is a number (decimals are allowed) and <unit> is 's, 'm', or 'h'")
				// ...or 'ns'/'µs' if that's your cup of tea.
			}
			return car.ScheduleSoftwareUpdate(ctx, delay)
		},
	},
	"software-update-cancel": {
		help:             "Cancel a pending software update",
		requiresAuth:     true,
		requiresFleetAPI: false,
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, _ map[string]string) error {
			return car.CancelSoftwareUpdate(ctx)
		},
	},
	"sentry-mode": {
		help:             "Set sentry mode to STATE ('on' or 'off')",
		requiresAuth:     true,
		requiresFleetAPI: false,
		args: []Argument{
			{name: "STATE", help: "'on' or 'off'"},
		},
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, args map[string]string) error {
			var state bool
			switch args["STATE"] {
			case "on":
				state = true
			case "off":
				state = false
			default:
				return fmt.Errorf("sentry mode state must be 'on' or 'off'")
			}
			return car.SetSentryMode(ctx, state)
		},
	},
	"wake": {
		help:             "Wake up vehicle",
		requiresAuth:     false,
		requiresFleetAPI: false,
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, _ map[string]string) error {
			return car.Wakeup(ctx)
		},
	},
	"tonneau-open": {
		help:             "Open Cybertruck tonneau.",
		requiresAuth:     true,
		requiresFleetAPI: false,
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, _ map[string]string) error {
			return car.OpenTonneau(ctx)
		},
	},
	"tonneau-close": {
		help:             "Close Cybertruck tonneau.",
		requiresAuth:     true,
		requiresFleetAPI: false,
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, _ map[string]string) error {
			return car.CloseTonneau(ctx)
		},
	},
	"tonneau-stop": {
		help:             "Stop moving Cybertruck tonneau.",
		requiresAuth:     true,
		requiresFleetAPI: false,
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, _ map[string]string) error {
			return car.StopTonneau(ctx)
		},
	},
	"trunk-open": {
		help:             "Open vehicle trunk. Note that trunk-close only works on certain vehicle types.",
		requiresAuth:     true,
		requiresFleetAPI: false,
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, _ map[string]string) error {
			return car.OpenTrunk(ctx)
		},
	},
	"trunk-move": {
		help:             "Toggle trunk open/closed. Closing is only available on certain vehicle types.",
		requiresAuth:     true,
		requiresFleetAPI: false,
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, _ map[string]string) error {
			return car.ActuateTrunk(ctx)
		},
	},
	"trunk-close": {
		help:             "Closes vehicle trunk. Only available on certain vehicle types.",
		requiresAuth:     true,
		requiresFleetAPI: false,
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, _ map[string]string) error {
			return car.CloseTrunk(ctx)
		},
	},
	"frunk-open": {
		help:             "Open vehicle frunk. Note that there's no frunk-close command!",
		requiresAuth:     true,
		requiresFleetAPI: false,
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, _ map[string]string) error {
			return car.OpenFrunk(ctx)
		},
	},
	"charge-port-open": {
		help:             "Open charge port",
		requiresAuth:     true,
		requiresFleetAPI: false,
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, _ map[string]string) error {
			return car.OpenChargePort(ctx)
		},
	},
	"charge-port-close": {
		help:             "Close charge port",
		requiresAuth:     true,
		requiresFleetAPI: false,
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, _ map[string]string) error {
			return car.CloseChargePort(ctx)
		},
	},
	"autosecure-modelx": {
		help:             "Close falcon-wing doors and lock vehicle. Model X only.",
		requiresAuth:     true,
		requiresFleetAPI: false,
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, _ map[string]string) error {
			return car.AutoSecureVehicle(ctx)
		},
	},
	"session-info": {
		help:             "Retrieve session info for PUBLIC_KEY from DOMAIN",
		requiresAuth:     false,
		requiresFleetAPI: false,
		args: []Argument{
			{name: "PUBLIC_KEY", help: "file containing public key (or corresponding private key)"},
			{name: "DOMAIN", help: "'vcsec' or 'infotainment'"},
		},
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, args map[string]string) error {
			// See SeatPosition definition for controlling backrest heaters (limited models).
			domains := map[string]protocol.Domain{
				"vcsec":        protocol.DomainVCSEC,
				"infotainment": protocol.DomainInfotainment,
			}
			domain, ok := domains[args["DOMAIN"]]
			if !ok {
				return fmt.Errorf("invalid domain %s", args["DOMAIN"])
			}
			publicKey, err := protocol.LoadPublicKey(args["PUBLIC_KEY"])
			if err != nil {
				return fmt.Errorf("invalid public key: %s", err)
			}
			info, err := car.SessionInfo(ctx, publicKey, domain)
			if err != nil {
				return err
			}
			fmt.Printf("%s\n", info)
			return nil
		},
	},
	"seat-heater": {
		help:             "Set seat heater at POSITION to LEVEL",
		requiresAuth:     true,
		requiresFleetAPI: false,
		args: []Argument{
			{name: "SEAT", help: "<front|2nd-row|3rd-row>-<left|center|right> (e.g., 2nd-row-left)"},
			{name: "LEVEL", help: "off, low, medium, or high"},
		},
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, args map[string]string) error {
			// See SeatPosition definition for controlling backrest heaters (limited models).
			seats := map[string]vehicle.SeatPosition{
				"front-left":     vehicle.SeatFrontLeft,
				"front-right":    vehicle.SeatFrontRight,
				"2nd-row-left":   vehicle.SeatSecondRowLeft,
				"2nd-row-center": vehicle.SeatSecondRowCenter,
				"2nd-row-right":  vehicle.SeatSecondRowRight,
				"3rd-row-left":   vehicle.SeatThirdRowLeft,
				"3rd-row-right":  vehicle.SeatThirdRowRight,
			}
			position, ok := seats[args["SEAT"]]
			if !ok {
				return fmt.Errorf("invalid seat position")
			}
			levels := map[string]vehicle.Level{
				"off":    vehicle.LevelOff,
				"low":    vehicle.LevelLow,
				"medium": vehicle.LevelMed,
				"high":   vehicle.LevelHigh,
			}
			level, ok := levels[args["LEVEL"]]
			if !ok {
				return fmt.Errorf("invalid seat heater level")
			}
			spec := map[vehicle.SeatPosition]vehicle.Level{
				position: level,
			}
			return car.SetSeatHeater(ctx, spec)
		},
	},
	"steering-wheel-heater": {
		help:             "Set steering wheel mode to STATE ('on' or 'off')",
		requiresAuth:     true,
		requiresFleetAPI: false,
		args: []Argument{
			{name: "STATE", help: "'on' or 'off'"},
		},
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, args map[string]string) error {
			var state bool
			switch args["STATE"] {
			case "on":
				state = true
			case "off":
				state = false
			default:
				return fmt.Errorf("steering wheel state must be 'on' or 'off'")
			}
			return car.SetSteeringWheelHeater(ctx, state)
		},
	},
	"product-info": {
		help:             "Print JSON product info",
		requiresAuth:     false,
		requiresFleetAPI: true,
		handler: func(ctx context.Context, acct *account.Account, _ *vehicle.Vehicle, _ map[string]string) error {
			productsJSON, err := acct.Get(ctx, "api/1/products")
			if err != nil {
				return err
			}
			fmt.Println(string(productsJSON))
			return nil
		},
	},
	"auto-seat-and-climate": {
		help:             "Turn on automatic seat heating and HVAC",
		requiresAuth:     true,
		requiresFleetAPI: false,
		args: []Argument{
			{name: "POSITIONS", help: "'L' (left), 'R' (right), or 'LR'"},
		},
		optional: []Argument{
			{name: "STATE", help: "'on' (default) or 'off'"},
		},
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, args map[string]string) error {
			var positions []vehicle.SeatPosition
			if strings.Contains(args["POSITIONS"], "L") {
				positions = append(positions, vehicle.SeatFrontLeft)
			}
			if strings.Contains(args["POSITIONS"], "R") {
				positions = append(positions, vehicle.SeatFrontRight)
			}
			if len(positions) != len(args["POSITIONS"]) {
				return fmt.Errorf("invalid seat position")
			}
			enabled := true
			if state, ok := args["STATE"]; ok && strings.ToUpper(state) == "OFF" {
				enabled = false
			}
			return car.AutoSeatAndClimate(ctx, positions, enabled)
		},
	},
	"windows-vent": {
		help:             "Vent all windows",
		requiresAuth:     true,
		requiresFleetAPI: false,
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, _ map[string]string) error {
			return car.VentWindows(ctx)
		},
	},
	"windows-close": {
		help:             "Close all windows",
		requiresAuth:     true,
		requiresFleetAPI: false,
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, _ map[string]string) error {
			return car.CloseWindows(ctx)
		},
	},
	"body-controller-state": {
		help:             "Fetch limited vehicle state information. Works over BLE when infotainment is asleep.",
		domain:           protocol.DomainVCSEC,
		requiresAuth:     false,
		requiresFleetAPI: false,
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, _ map[string]string) error {
			info, err := car.BodyControllerState(ctx)
			if err != nil {
				return err
			}
			options := protojson.MarshalOptions{
				Indent:            "\t",
				UseEnumNumbers:    false,
				EmitUnpopulated:   false,
				EmitDefaultValues: true,
			}
			fmt.Println(options.Format(info))
			return nil
		},
	},
	"guest-mode-on": {
		help:             "Enable Guest Mode. See https://developer.tesla.com/docs/fleet-api/endpoints/vehicle-commands#guest-mode.",
		requiresAuth:     true,
		requiresFleetAPI: false,
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, _ map[string]string) error {
			return car.SetGuestMode(ctx, true)
		},
	},
	"guest-mode-off": {
		help:             "Disable Guest Mode.",
		requiresAuth:     true,
		requiresFleetAPI: false,
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, _ map[string]string) error {
			return car.SetGuestMode(ctx, false)
		},
	},
	"erase-guest-data": {
		help:             "Erase Guest Mode user data",
		requiresAuth:     true,
		requiresFleetAPI: false,
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, _ map[string]string) error {
			return car.EraseGuestData(ctx)
		},
	},
	"charging-schedule-add": {
		help:             "Schedule charge for DAYS START_TIME-END_TIME at LATITUDE LONGITUDE. The END_TIME may be on the following day.",
		requiresAuth:     true,
		requiresFleetAPI: false,
		args: []Argument{
			{name: "DAYS", help: "Comma-separated list of any of Sun, Mon, Tues, Wed, Thurs, Fri, Sat OR all OR weekdays"},
			{name: "TIME", help: "Time interval to charge (24-hour clock). Examples: '22:00-6:00', '-6:00', '20:32-"},
			{name: "LATITUDE", help: "Latitude of charging site"},
			{name: "LONGITUDE", help: "Longitude of charging site"},
		},
		optional: []Argument{
			{name: "REPEAT", help: "Set to 'once' or omit to repeat weekly"},
			{name: "ID", help: "The ID of the charge schedule to modify. Not required for new schedules."},
			{name: "ENABLED", help: "Whether the charge schedule is enabled. Expects 'true' or 'false'. Defaults to true."},
		},
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, args map[string]string) error {
			var err error
			schedule := vehicle.ChargeSchedule{
				Id:      uint64(time.Now().Unix()),
				Enabled: true,
			}

			if enabledStr, ok := args["ENABLED"]; ok {
				schedule.Enabled = enabledStr == "true"
			}

			schedule.DaysOfWeek, err = GetDays(args["DAYS"])
			if err != nil {
				return err
			}

			r := strings.Split(args["TIME"], "-")
			if len(r) != 2 {
				return errors.New("invalid time range")
			}

			if r[0] != "" {
				schedule.StartTime, err = MinutesAfterMidnight(r[0])
				schedule.StartEnabled = true
				if err != nil {
					return err
				}
			}

			if r[1] != "" {
				schedule.EndTime, err = MinutesAfterMidnight(r[1])
				schedule.EndEnabled = true
				if err != nil {
					return err
				}
			}

			schedule.Latitude, err = GetDegree(args["LATITUDE"])
			if err != nil {
				return err
			}

			schedule.Longitude, err = GetDegree(args["LONGITUDE"])
			if err != nil {
				return err
			}

			if repeatPolicy, ok := args["REPEAT"]; ok && repeatPolicy == "once" {
				schedule.OneTime = true
			}

			if err := car.AddChargeSchedule(ctx, &schedule); err != nil {
				return err
			}
			fmt.Printf("%d\n", schedule.Id)
			return nil
		},
	},
	"charging-schedule-remove": {
		help:             "Removes charging schedule of TYPE [ID]",
		requiresAuth:     true,
		requiresFleetAPI: false,
		args: []Argument{
			{name: "TYPE", help: "home|work|other|id"},
		},
		optional: []Argument{
			{name: "ID", help: "numeric ID of schedule to remove when TYPE set to id"},
		},
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, args map[string]string) error {
			var home, work, other bool
			switch strings.ToUpper(args["TYPE"]) {
			case "ID":
				if idStr, ok := args["ID"]; ok {
					id, err := strconv.ParseUint(idStr, 10, 64)
					if err != nil {
						return errors.New("expected numeric ID")
					}
					return car.RemoveChargeSchedule(ctx, id)
				} else {
					return errors.New("missing schedule ID")
				}
			case "HOME":
				home = true
			case "WORK":
				work = true
			case "OTHER":
				other = true
			default:
				return errors.New("TYPE must be home|work|other|id")
			}
			return car.BatchRemoveChargeSchedules(ctx, home, work, other)
		},
	},
	"precondition-schedule-add": {
		help:             "Schedule precondition for DAYS TIME at LATITUDE LONGITUDE.",
		requiresAuth:     true,
		requiresFleetAPI: false,
		args: []Argument{
			{name: "DAYS", help: "Comma-separated list of any of Sun, Mon, Tues, Wed, Thurs, Fri, Sat OR all OR weekdays"},
			{name: "TIME", help: "Time to precondition by. Example: '22:00'"},
			{name: "LATITUDE", help: "Latitude of location to precondition at."},
			{name: "LONGITUDE", help: "Longitude of location to precondition at."},
		},
		optional: []Argument{
			{name: "REPEAT", help: "Set to 'once' or omit to repeat weekly"},
			{name: "ID", help: "The ID of the precondition schedule to modify. Not required for new schedules."},
			{name: "ENABLED", help: "Whether the precondition schedule is enabled. Expects 'true' or 'false'. Defaults to true."},
		},
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, args map[string]string) error {
			var err error
			schedule := vehicle.PreconditionSchedule{
				Id:      uint64(time.Now().Unix()),
				Enabled: true,
			}

			if enabledStr, ok := args["ENABLED"]; ok {
				schedule.Enabled = enabledStr == "true"
			}

			if idStr, ok := args["ID"]; ok {
				id, err := strconv.ParseUint(idStr, 10, 64)
				if err != nil {
					return errors.New("expected numeric ID")
				}
				schedule.Id = id
			}

			schedule.DaysOfWeek, err = GetDays(args["DAYS"])
			if err != nil {
				return err
			}

			if timeStr, ok := args["TIME"]; ok {
				schedule.PreconditionTime, err = MinutesAfterMidnight(timeStr)
				if err != nil {
					return err
				}
			} else {
				return errors.New("expected TIME")
			}

			schedule.Latitude, err = GetDegree(args["LATITUDE"])
			if err != nil {
				return err
			}

			schedule.Longitude, err = GetDegree(args["LONGITUDE"])
			if err != nil {
				return err
			}

			if repeatPolicy, ok := args["REPEAT"]; ok && repeatPolicy == "once" {
				schedule.OneTime = true
			}

			if err := car.AddPreconditionSchedule(ctx, &schedule); err != nil {
				return err
			}
			fmt.Printf("%d\n", schedule.Id)
			return nil
		},
	},
	"precondition-schedule-remove": {
		help:             "Removes precondition schedule of TYPE [ID]",
		requiresAuth:     true,
		requiresFleetAPI: false,
		args: []Argument{
			{name: "TYPE", help: "home|work|other|id"},
		},
		optional: []Argument{
			{name: "ID", help: "numeric ID of schedule to remove when TYPE set to id"},
		},
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, args map[string]string) error {
			var home, work, other bool
			switch strings.ToUpper(args["TYPE"]) {
			case "ID":
				if idStr, ok := args["ID"]; ok {
					id, err := strconv.ParseUint(idStr, 10, 64)
					if err != nil {
						return errors.New("expected numeric ID")
					}
					return car.RemovePreconditionSchedule(ctx, id)
				} else {
					return errors.New("missing schedule ID")
				}
			case "HOME":
				home = true
			case "WORK":
				work = true
			case "OTHER":
				other = true
			default:
				return errors.New("TYPE must be home|work|other|id")
			}
			return car.BatchRemovePreconditionSchedules(ctx, home, work, other)
		},
	},
	"state": {
		help:             "Fetch vehicle state over BLE.",
		requiresAuth:     true,
		requiresFleetAPI: false,
		args: []Argument{
			{name: "CATEGORY", help: "One of " + strings.Join(categoryNames(), ", ")},
		},
		handler: func(ctx context.Context, _ *account.Account, car *vehicle.Vehicle, args map[string]string) error {
			category, err := GetCategory(args["CATEGORY"])
			if err != nil {
				return err
			}
			data, err := car.GetState(ctx, category)
			if err != nil {
				return err
			}
			fmt.Println(protojson.Format(data))
			return nil
		},
	},
}
