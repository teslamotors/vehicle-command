package protocol

import (
	"errors"
	"fmt"
	"strings"

	"google.golang.org/protobuf/proto"

	"github.com/teslamotors/vehicle-command/internal/authentication"
	verror "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/errors"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/signatures"
	universal "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/universalmessage"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/vcsec"
)

// Error exposes methods useful for categorizing errors.
type Error interface {
	error

	// MayHaveSucceeded returns true if the Error was triggered a command that might have been executed.
	// For example, if a client times out while waiting for a response, then the client cannot tell
	// if the command was received. (Not all timeouts mean the command MayHaveSucceeded, so the
	// common Timeout() error interface is not appropriate here).
	MayHaveSucceeded() bool

	// Temporary returns true if the Error might be the result of a transient condition. For
	// example, it's not unusual for the car to return Busy errors if it's in the process of waking
	// from sleep and the services responsible for executing the command are not yet running.
	Temporary() bool
}

var (
	// ErrBusy indicates a resource is temporarily unavailable.
	ErrBusy = NewError("vehicle busy or finishing wake-up", false, true)
	// ErrUnknown indicates the client received an unrecognized error code. Check for package
	// updates.
	ErrUnknown = NewError("vehicle responded with an unrecognized status code", false, false)
	// ErrNotConnected indicates the vehicle could not be reached.
	ErrNotConnected = NewError("vehicle not connected", false, false)
	// ErrNoSession indicates the client has not established a session with the vehicle. You may
	// have forgotten to call vehicle.StartSessions(...).
	ErrNoSession = NewError("cannot send authenticated command before establishing a vehicle session", false, false)
	// ErrRequiresKey indicates a client tried to send a command without an ECDHPrivateKey.
	ErrRequiresKey = NewError("no private key available", false, false)
	// ErrInvalidPublicKey indicates a client tried to perform an operation with an invalid public
	// key. Public keys are NIST-P256 EC keys, encoded in uncompressed form.
	ErrInvalidPublicKey     = authentication.ErrInvalidPublicKey
	ErrKeyNotPaired         = NewError("vehicle rejected request: your public key has not been paired with the vehicle", false, false)
	ErrUnpexpectedPublicKey = errors.New("remote public key changed unexpectedly")
	ErrBadResponse          = errors.New("invalid response")
	ErrProtocolNotSupported = errors.New("vehicle does not support protocol -- use REST API")
	ErrRequiresBLE          = errors.New("command can only be sent over BLE")
	// ErrKeyNameRequiresFleetAPI indicates a client asked to set the human-readable
	// key label shown on the vehicle Locks screen. That string is account
	// metadata at api/1/users/keys, not a VCSEC field. KeyMetadata on the
	// vehicle only stores form factor. BLE-only apps cannot rename a key;
	// use [github.com/teslamotors/vehicle-command/pkg/account.Account.UpdateKey]
	// (tesla-control rename-key without -ble). See teslamotors/vehicle-command#418.
	ErrKeyNameRequiresFleetAPI = NewError("key display names are stored by Fleet API (api/1/users/keys), not on the vehicle; BLE cannot set them", false, false)
	// ErrWiFiNotInProtocol indicates a client asked to enable/disable WiFi, add or
	// forget a network, or toggle connect-in-drive. Those actions exist in the
	// in-car UX but Tesla has not published VehicleAction field numbers for them
	// (they are also absent from public firmware dumps). Guessing unused oneof
	// numbers would collide with firmware and could leak a PSK on the wire.
	// Connectivity telemetry is teslamotors/fleet-telemetry#407, not this SDK.
	// See teslamotors/vehicle-command#419.
	ErrWiFiNotInProtocol = NewError("WiFi configuration is not in the published vehicle command protocol (no VehicleAction for enable, add-network, forget, or connect-in-drive); see teslamotors/vehicle-command#419", false, false)
	// ErrKeepAwakeNotInProtocol indicates a client asked to inhibit infotainment
	// sleep (keep the car "awake") without Sentry Mode. wake / RKE_ACTION_WAKE_VEHICLE
	// starts infotainment but does not prevent later sleep. SetKeepAccessoryPowerMode
	// powers the 12V jack and charging USB ports, not the glovebox dashcam/data
	// USB. Tesla has not published a VehicleAction for a keep-alive. Wrapping an
	// Infotainment mutation (for example charge-port-close) as a library keep-alive
	// would change vehicle state and fight designed sleep. See teslamotors/vehicle-command#397.
	ErrKeepAwakeNotInProtocol = NewError("the published vehicle command protocol has no keep-awake action; wake starts infotainment but does not inhibit sleep, and keep-accessory-power does not power the glovebox dashcam USB; see teslamotors/vehicle-command#397", false, false)
	// ErrBatteryOptionRequiresFleetAPI indicates a client asked the signed
	// vehicle protocol (or BLE) for pack identity / battery size. $BT* codes
	// are Tesla catalog metadata at GET /api/1/dx/vehicles/options, not a
	// VehicleAction or ChargeState field. See teslamotors/vehicle-command#391.
	ErrBatteryOptionRequiresFleetAPI = NewError("battery option codes are Tesla catalog metadata (GET /api/1/dx/vehicles/options), not a signed vehicle command; BLE cannot fetch them. See teslamotors/vehicle-command#391", false, false)
	// ErrBatteryOptionNotInCatalog indicates Tesla's options response for this
	// VIN omitted a battery ($BT*) code. This SDK does not invent codes from
	// model options such as $MT322. ChargeState has SOC/range, not pack kWh.
	// Partner GET /api/1/vehicles/{vin}/specs (batteryCapacityKwh) is Tesla's
	// billed alternative. See teslamotors/vehicle-command#391.
	ErrBatteryOptionNotInCatalog = NewError("Tesla's vehicle options catalog did not include a battery ($BT*) option code; this SDK does not invent them. ChargeState has SOC/range, not pack kWh. Partner GET /api/1/vehicles/{vin}/specs (batteryCapacityKwh) is billed. See teslamotors/vehicle-command#391", false, false)
	// ErrBLEKeyPresenceNotInProtocol indicates a client asked to enroll a BLE
	// device that can authorize commands but is ignored for Walk-Away Door Lock
	// / passive-entry presence. add-key-request puts the public key on the
	// VCSEC whitelist; that is a vehicle key. KeyMetadata stores only form
	// factor, and Role only gates commands. Tesla has not published a whitelist
	// flag that exempts an enrolled BLE client from key-presence. Leave the
	// GATT session connected (or reconnect periodically) inside the cabin and
	// the vehicle can treat the device like a phone key left behind. The
	// supported in-car architecture is connect, command, Disconnect. See
	// teslamotors/vehicle-command#480.
	ErrBLEKeyPresenceNotInProtocol = NewError("an enrolled BLE client is a VCSEC whitelist key; Tesla has not published a flag to ignore it for Walk-Away Door Lock. Disconnect after commands. See teslamotors/vehicle-command#480", false, false)
	// ErrScheduledChargingFirmware indicates a client asked this SDK to make
	// scheduled charging or scheduled departure fire while cabin overheat
	// protection is on. ScheduledChargingAction and ScheduledDepartureAction
	// are already published and delivered. Whether the vehicle later sleeps
	// and runs the scheduler is firmware. On some Intel-MCU Model S vehicles,
	// cabin overheat protection can prevent that sleep/wake cycle. Regular
	// charge-start still works. This library does not disable cabin overheat
	// as a workaround. See teslamotors/vehicle-command#342.
	ErrScheduledChargingFirmware = NewError("scheduled charging/departure commands are delivered by this SDK; whether the vehicle later sleeps and fires the scheduler is firmware. Cabin overheat protection can prevent that cycle on some Intel-MCU Model S vehicles. This library does not disable cabin overheat as a workaround. See teslamotors/vehicle-command#342", false, false)
	// ErrBoomboxNotInProtocol indicates a client asked to play the external
	// speaker (Fleet API remote_boombox / "fart" / locate ping). Fleet API
	// still lists the REST path, but Tesla has not published a VehicleAction
	// for it. Honk and flash-lights are published (fields 27 and 26). A Tesla
	// collaborator stated boombox stays unpublished pending legal approval
	// because Pedestrian Warning System use is restricted by jurisdiction
	// (teslamotors/vehicle-command#266). Guessing an unused oneof number, or
	// copying a field from a third-party firmware dump, would collide with
	// firmware and release code this repository is not cleared to ship.
	// Unsigned REST is rejected with 403 Vehicle Command Protocol required.
	// See teslamotors/vehicle-command#266 and #411.
	ErrBoomboxNotInProtocol = NewError("remote_boombox is not in the published vehicle command protocol (no VehicleAction; Tesla has not released it pending legal review of Pedestrian Warning System restrictions). Do not invent or copy a field number. See teslamotors/vehicle-command#266", false, false)
	// ErrHvacAutoModeNotInProtocol indicates a client asked to switch HVAC
	// Auto vs Manual mode (or heater-off / vent-only as in #112). HvacAutoAction
	// is climate power on/off (Fleet auto_conditioning_start / stop). Its
	// manual_override bit is a low-SOC override, not Auto vs Manual. Climate
	// setpoints must use driver_temp_celsius and passenger_temp_celsius
	// (ChangeClimateTemp / set_temps); absolute_celsius alone leaves those
	// proto3 zeros, which firmware treats as LO. Tesla has not published a
	// VehicleAction for Auto vs Manual HVAC. Guessing an unused oneof number
	// would collide with firmware. See teslamotors/vehicle-command#283.
	ErrHvacAutoModeNotInProtocol = NewError("HVAC Auto vs Manual mode is not in the published vehicle command protocol (HvacAutoAction is climate on/off; manual_override is a low-SOC override). Do not invent a field number. Set temperatures with driver_temp_celsius and passenger_temp_celsius. See teslamotors/vehicle-command#283", false, false)
	// ErrClimateSplitNotInProtocol indicates a client asked to toggle the
	// in-car climate split / SYNC control (linked vs independent driver and
	// passenger HVAC) or to read a dedicated split boolean from ClimateState.
	// HvacTemperatureAdjustmentAction already sets driver_temp_celsius and
	// passenger_temp_celsius over BLE (climate-set-temp / set_temps).
	// ClimateState has driver_temp_setting and passenger_temp_setting, not a
	// published is_climate_split / is_sync field. Inferring split from unequal
	// setpoints is not the UI SYNC flag. Tesla has not published a VehicleAction
	// for that toggle. Guessing an unused oneof number, or adding a ClimateState
	// field Tesla has not shipped, would collide with firmware. See
	// teslamotors/vehicle-command#386.
	ErrClimateSplitNotInProtocol = NewError("climate split/SYNC is not in the published vehicle command protocol (no VehicleAction or ClimateState split boolean). Independent setpoints are HvacTemperatureAdjustmentAction driver_temp_celsius / passenger_temp_celsius. Do not invent a field number. See teslamotors/vehicle-command#386", false, false)
	// ErrChargingManagerChargePortFirmware indicates a client asked this SDK
	// to grant ROLE_CHARGING_MANAGER the ability to open or close the charge
	// port. ChargePortDoorOpen/Close are already published and delivered
	// (OpenChargePort / tesla-control charge-port-open). Role checks are
	// firmware. Charging Manager can charging-start/stop/set-amps over BLE;
	// charge-port historically returns MESSAGEFAULT_ERROR_INSUFFICIENT_PRIVILEGES
	// (teslamotors/vehicle-command#232). Tesla has not expanded that ACL.
	// This library does not enroll Owner, rewrite the command as a VCSEC
	// ClosureMoveRequest.chargePort bypass, or invent extra KeyMetadata
	// permissions. See teslamotors/vehicle-command#413.
	ErrChargingManagerChargePortFirmware = NewError("Charging Manager can authorize charging commands; charge-port open/close is firmware-gated (MESSAGEFAULT_ERROR_INSUFFICIENT_PRIVILEGES). This SDK still delivers ChargePortDoorOpen/Close and does not enroll Owner as a workaround. See teslamotors/vehicle-command#413", false, false)
	// ErrBLEStateLatencyFirmware indicates a client asked this SDK to guarantee
	// GetDriveState / GetVehicleData over BLE faster than the vehicle round-trip
	// (reported ~250–300ms; targets of <150ms or 50–100ms). Construction and
	// encryption are a few milliseconds; the rest is Infotainment processing
	// plus BLE. There is no published streaming DriveState VehicleAction.
	// Disabling FLAG_ENCRYPT_RESPONSE, shortening UUIDs, or skipping the
	// handshake does not raise a firmware poll-rate cap and is unsafe.
	// Handshake once and reuse the session; each extra GetState category is
	// another RTT. High-rate telemetry is teslamotors/fleet-telemetry, not
	// this repo. See teslamotors/vehicle-command#414.
	ErrBLEStateLatencyFirmware = NewError("BLE GetDriveState latency is a vehicle round-trip (~250-300ms observed), not a client timer. This SDK cannot guarantee <150ms, does not disable response encryption, and has no streaming DriveState action. Reuse the session; fleet-telemetry is a separate product. See teslamotors/vehicle-command#414", false, false)
	// ErrSeatClimateFleetAPI indicates a client treated Tesla signed_command
	// HTTP 501 Unauthorized as tesla-http-proxy missing remote_seat_heater_request
	// / remote_seat_cooler_request. Those REST paths already map to published
	// HvacSeatHeaterActions (field 36) and HvacSeatCoolerActions (field 49).
	// tesla-control seat-heater / seat-cooler send them. writeJSONError copies
	// Tesla's HTTP status, so StatusText(501) is "Not Implemented" even though
	// ExtractCommandAction succeeded and the proxy POSTed signed_command.
	// JSON error "Unauthorized" is Tesla Fleet API partner/region/OAuth
	// allowlist. This library does not invent unused VehicleAction numbers or
	// skip command signing. See teslamotors/vehicle-command#383.
	ErrSeatClimateFleetAPI = NewError("remote_seat_heater_request and remote_seat_cooler_request are implemented (HvacSeatHeaterActions / HvacSeatCoolerActions). HTTP 501 Unauthorized from Tesla signed_command is Fleet API partner/region/OAuth allowlist, not a missing proxy handler. tesla-http-proxy forwards Tesla's status (Not Implemented). See teslamotors/vehicle-command#383", false, false)
	// ErrPartnerOAuthNotProvisioned indicates Tesla Fleet Auth rejected
	// client_credentials with invalid_audience, or /authorize showed
	// "No policy rules". tesla-auth-token only stores a token the caller
	// already obtained; account.New only reads the JWT aud claim of that
	// token. This SDK does not mint partner tokens, bind OAuth audiences, or
	// POST /api/1/partner_accounts without a token. A dashboard "Active" app
	// can still lack Tesla IdP policy/audience. Trying every regional
	// fleet-api audience does not provision it. Use Tesla developer dashboard
	// Support Inquiry. See teslamotors/vehicle-command#460.
	ErrPartnerOAuthNotProvisioned = NewError("Tesla Fleet Auth invalid_audience and /authorize \"No policy rules\" mean Tesla has not bound OAuth policy/audience to the application. This SDK stores tokens (tesla-auth-token) and does not mint partner tokens or register partner_accounts. Use Tesla developer dashboard Support Inquiry. See teslamotors/vehicle-command#460", false, false)
	// ErrChargingWhileInfotainmentAsleep indicates a client asked this SDK to
	// treat Fleet Telemetry (ACChargingPower, Soc) as proof Infotainment will
	// accept signed_command, or to invent keep-awake / a charging-controller
	// bypass so charge_stop and set_charging_amps work while Tesla returns
	// "vehicle unavailable: vehicle is offline or asleep". Those REST paths
	// already map to published Infotainment VehicleActions
	// (ChargingStartStopAction stop, SetChargingAmpsAction). Charging hardware
	// can run while Infotainment is asleep, so telemetry can look live while
	// Tesla Fleet API HTTP 408/503 maps to inet.ErrVehicleNotAwake. wake starts
	// Infotainment but does not inhibit later sleep (#397) and does not
	// guarantee Tesla's command gateway is ready. Paid Fleet API usage is not
	// a quota on this path. BLE still delivers these actions to Infotainment;
	// it is not a VCSEC charging-amps command. See teslamotors/vehicle-command#452.
	ErrChargingWhileInfotainmentAsleep = NewError("Fleet Telemetry can report charging while Infotainment is asleep. charge_stop and set_charging_amps are Infotainment VehicleActions; Tesla signed_command returns vehicle unavailable (offline or asleep) when Infotainment is unreachable. wake does not inhibit sleep. This SDK does not invent keep-awake or a charging-controller bypass. See teslamotors/vehicle-command#452", false, false)
	// ErrInfotainmentSignedCommandOffline indicates a client asked this SDK
	// to treat GET /api/1/vehicles state=online, or a successful
	// DOMAIN_VEHICLE_SECURITY signed_command, as proof DOMAIN_INFOTAINMENT
	// will accept the next session_info_request. Both domains are posted to
	// the same signed_command endpoint; the domain is inside the protobuf.
	// Tesla's gateway can still return HTTP 408 "vehicle is offline"
	// (inet.ErrVehicleNotAwake) for Infotainment while the vehicle list says
	// online, VCSEC commands succeed, BLE reaches both domains, and the
	// operator is in the car, reported on fleet-api.prd.cn
	// (teslamotors/vehicle-command#285). wake_up is unsigned REST and does
	// not guarantee the next Infotainment handshake. This library does not
	// rewrite the domain to VCSEC, skip the Infotainment handshake
	// (#468 / closed PR #470), or invent a second transport inside the proxy.
	ErrInfotainmentSignedCommandOffline = NewError("GET /api/1/vehicles state=online and a successful VCSEC signed_command do not mean DOMAIN_INFOTAINMENT signed_command will succeed. Tesla's gateway can return HTTP 408 vehicle is offline for Infotainment (reported on fleet-api.prd.cn while the car is occupied and BLE works for both domains). This SDK does not rewrite the domain or skip the Infotainment handshake. See teslamotors/vehicle-command#285", false, false)
	// ErrClimateKeeperCPDFirmware indicates a client asked this SDK to enable
	// Dog or Camp mode despite firmware refusing with NominalError
	// "cpd_enabled", or to treat the in-car Child Left Alone Detection
	// setting as a CPD override. set_climate_keeper_mode is already published
	// (HvacClimateKeeperAction field 44; Dog=2, Camp=3). tesla-http-proxy
	// delivers it; HTTP 200 result:false with reason cpd_enabled is an
	// application-layer refusal from the car, not a missing handler.
	// cpd is Child Presence Detection (occupancy firmware), not the Child
	// Left Alone Detection UX toggle. HvacClimateKeeperAction.manual_override
	// is a low-SOC override, not a CPD bypass
	// (teslamotors/vehicle-command#437). Tesla has not published a
	// VehicleAction to disable CPD, a confirmation flow, or an extra OAuth
	// scope for third-party Dog/Camp. This library still delivers the
	// published action so a future firmware grant works without an SDK
	// change. See teslamotors/vehicle-command#509.
	ErrClimateKeeperCPDFirmware = NewError("set_climate_keeper_mode is published (HvacClimateKeeperAction Dog/Camp). Firmware may refuse with cpd_enabled (Child Presence Detection occupancy), which is not the Child Left Alone Detection setting. manual_override is a low-SOC override, not a CPD bypass. This SDK does not invent a CPD-disable action. See teslamotors/vehicle-command#509", false, false)
	// ErrVirtualKeyReturnURI indicates a client asked this SDK to change the
	// Finish Setup button on Tesla's hosted virtual-key page
	// (https://tesla.com/_ak/<domain>) or to append an arbitrary return_uri.
	// That page is Tesla's website, not this repository. An unconstrained
	// return URL is an open redirect. A Tesla collaborator said any redirect
	// must stay on the registered partner domain and/or be configured in
	// advance (teslamotors/vehicle-command#444). This SDK builds the
	// enrollment link without a query string and does not host a substitute
	// page.
	ErrVirtualKeyReturnURI = NewError("the virtual key Finish Setup button is on Tesla's hosted https://tesla.com/_ak/<domain> page, not in this SDK. An unconstrained return_uri is an open redirect; any redirect must stay on the registered partner domain and be configured with Tesla in advance. This SDK does not append return_uri. See teslamotors/vehicle-command#444", false, false)
	ErrRequiresEncryption  = errors.New("command should not be sent in plaintext or encrypted with an unauthenticated public key")
	ErrNoDecryptionContext = errors.New("could not decrypt vehicle response without a session")
	// ErrReplayedResponse indicates the client received multiple responses from the vehicle with
	// the same response counter. This could be benign, as the network may have reattempted
	// transmission.
	ErrReplayedResponse = errors.New("received vehicle response with duplicate counter")
)

type CommandError struct {
	Err               error
	PossibleSuccess   bool
	PossibleTemporary bool
}

func NewError(message string, mayHaveSucceeded bool, temporary bool) error {
	return &CommandError{Err: errors.New(message), PossibleSuccess: mayHaveSucceeded, PossibleTemporary: temporary}
}

func (e *CommandError) Error() string {
	return e.Err.Error()
}

func (e *CommandError) Unwrap() error {
	return e.Err
}

func (e *CommandError) MayHaveSucceeded() bool {
	return e.PossibleSuccess
}

func (e *CommandError) Temporary() bool {
	return e.PossibleTemporary
}

// KeychainError represents an error that occurred while trying to modify a vehicle's keychain.
type KeychainError struct {
	Code vcsec.WhitelistOperationInformation_E
}

func (e *KeychainError) MayHaveSucceeded() bool {
	return false
}

func (e *KeychainError) Temporary() bool {
	return false
}

func (e *KeychainError) Error() string {
	return fmt.Sprintf("keychain operation failed: %s", e.Code)
}

// MayHaveSucceeded returns true if err is a CommandError that indicates the command may have been
// executed but the client did not receive a confirmation from the vehicle.
func MayHaveSucceeded(err error) bool {
	var commErr Error
	if errors.As(err, &commErr) && commErr.MayHaveSucceeded() {
		return true
	}
	return false
}

// Temporary returns true if err is a CommandError that indicates the command failed due to possibly
// transient conditions that do not require user action to resolve.
func Temporary(err error) bool {
	var commErr Error
	if errors.As(err, &commErr) && commErr.Temporary() {
		return true
	}
	return false
}

// ShouldRetry returns true if the client should retry to issue the command that triggered an error.
func ShouldRetry(err error) bool {
	if err == nil {
		return false
	}
	var e Error
	if errors.As(err, &e) {
		if e.MayHaveSucceeded() {
			return false
		}
		if e.Temporary() {
			return true
		}
	}
	return false
}

// NominalVCSECError indicates the vehicle received and authenticated a command, but could not
// execute it.
type NominalError struct {
	Details error
}

func (e *NominalError) Error() string {
	return e.Details.Error()
}

func (e *NominalError) Unwrap() error {
	return e.Details
}

func (e *NominalError) MayHaveSucceeded() bool {
	return MayHaveSucceeded(e.Details)
}

func (e *NominalError) Temporary() bool {
	return Temporary(e.Details)
}

func IsNominalError(err error) bool {
	if err == nil {
		return false
	}
	var nErr *NominalError
	return errors.As(err, &nErr)
}

// IsClimateKeeperCPDEnabled reports whether err is a Dog/Camp refusal because
// Child Presence Detection is active. Live vehicles return HTTP 200 with
// result:false and NominalError "car could not execute command: cpd_enabled".
// Callers that asked this SDK to bypass CPD get ErrClimateKeeperCPDFirmware.
// See teslamotors/vehicle-command#509 and #437.
func IsClimateKeeperCPDEnabled(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrClimateKeeperCPDFirmware) {
		return true
	}
	if !IsNominalError(err) {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "cpd_enabled")
}

// NominalVCSECError indicates the vehicle security controller received and authenticated a command,
// but could not execute it.
type NominalVCSECError struct {
	Details *verror.NominalError
}

func (n *NominalVCSECError) Error() string {
	// This is future proofing in case other error types are added
	if n.Details.GetGenericError() != verror.GenericError_E_GENERICERROR_NONE {
		return "vcsec could not execute command: " + n.Details.String()
	}
	return "vcsec could not execute command: " + n.Details.GetGenericError().String()
}

func (n *NominalVCSECError) MayHaveSucceeded() bool {
	return false
}

func (n *NominalVCSECError) Temporary() bool {
	return false
}

// RoutableMessageError represents a protocol-layer error.
//
// Callers can inspect Code to react to specific faults:
//
//	var rmErr *protocol.RoutableMessageError
//	if errors.As(err, &rmErr) && rmErr.Code == universal.MessageFault_E_MESSAGEFAULT_ERROR_RESPONSE_MTU_EXCEEDED {
//		// The vehicle processed the request but could not fit the reply in a message.
//	}
type RoutableMessageError struct {
	Code universal.MessageFault_E
}

// messageFaultDescriptions supplements the raw MessageFault_E enum names with an explanation of
// what happened and what (if anything) the client can do about it. Only faults whose names are
// not self-explanatory need an entry.
var messageFaultDescriptions = map[universal.MessageFault_E]string{
	universal.MessageFault_E_MESSAGEFAULT_ERROR_REQUEST_MTU_EXCEEDED: "the request contained a field that exceeds " +
		"the vehicle's maximum message size and was not processed",
	// The vehicle enforces its own ceiling on the size of a serialized response (roughly 450 bytes
	// over BLE at the time of writing). This is a vehicle-side memory budget, not the negotiated
	// link MTU, so reconnecting or renegotiating the MTU does not help. See
	// https://github.com/teslamotors/vehicle-command/issues/472.
	universal.MessageFault_E_MESSAGEFAULT_ERROR_RESPONSE_MTU_EXCEEDED: "the vehicle received the request but its " +
		"response exceeded the vehicle's maximum message size and was dropped; the limit is enforced by the " +
		"vehicle and cannot be raised by the client",
}

func (v *RoutableMessageError) MayHaveSucceeded() bool {
	return v.Code == universal.MessageFault_E_MESSAGEFAULT_ERROR_NONE ||
		v.Code == universal.MessageFault_E_MESSAGEFAULT_ERROR_RESPONSE_MTU_EXCEEDED
}

// retriableErrors can sometimes be remedied if the client retries the command,
// possibly after using an error message to update session state.
var retriableErrors = []universal.MessageFault_E{
	universal.MessageFault_E_MESSAGEFAULT_ERROR_BUSY,
	universal.MessageFault_E_MESSAGEFAULT_ERROR_TIMEOUT,
	universal.MessageFault_E_MESSAGEFAULT_ERROR_INVALID_SIGNATURE,
	universal.MessageFault_E_MESSAGEFAULT_ERROR_INVALID_TOKEN_OR_COUNTER,
	universal.MessageFault_E_MESSAGEFAULT_ERROR_INTERNAL,
	universal.MessageFault_E_MESSAGEFAULT_ERROR_INCORRECT_EPOCH,
	universal.MessageFault_E_MESSAGEFAULT_ERROR_TIME_EXPIRED,
	universal.MessageFault_E_MESSAGEFAULT_ERROR_TIME_TO_LIVE_TOO_LONG,
}

func (v *RoutableMessageError) Temporary() bool {
	for _, code := range retriableErrors {
		if v.Code == code {
			return true
		}
	}
	return false
}

func (v *RoutableMessageError) Error() string {
	errString, ok := universal.MessageFault_E_name[int32(v.Code)]
	if !ok {
		return fmt.Sprintf("unrecognized error code %d", v.Code)
	}
	if description, ok := messageFaultDescriptions[v.Code]; ok {
		return errString + ": " + description
	}
	return errString
}

// GetError translates a universal.RoutableMessage into an appropriate Error,
// returning nil if the universal.RoutableMessage did not contain an error.
func GetError(u *universal.RoutableMessage) error {
	if fault := u.GetSignedMessageStatus().GetSignedMessageFault(); fault != universal.MessageFault_E_MESSAGEFAULT_ERROR_NONE {
		// This fault is relatively common but doesn't have a very enlightening error message, so we
		// override it with a more descriptive one.
		if fault == universal.MessageFault_E_MESSAGEFAULT_ERROR_UNKNOWN_KEY_ID {
			return ErrKeyNotPaired
		}
		return &RoutableMessageError{Code: fault}
	}
	if encodedSessionInfo := u.GetSessionInfo(); encodedSessionInfo != nil {
		var sessionInfo signatures.SessionInfo
		if err := proto.Unmarshal(encodedSessionInfo, &sessionInfo); err != nil {
			return ErrBadResponse
		}
		switch sessionInfo.GetStatus() {
		case signatures.Session_Info_Status_SESSION_INFO_STATUS_OK:
			break
		case signatures.Session_Info_Status_SESSION_INFO_STATUS_KEY_NOT_ON_WHITELIST:
			return ErrKeyNotPaired
		default:
			return ErrUnknown
		}
	}
	switch u.GetSignedMessageStatus().GetOperationStatus() {
	case universal.OperationStatus_E_OPERATIONSTATUS_OK:
		return nil
	case universal.OperationStatus_E_OPERATIONSTATUS_WAIT:
		return ErrBusy
	case universal.OperationStatus_E_OPERATIONSTATUS_ERROR:
	default:
		return ErrUnknown
	}
	return nil
}
