package protocol

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	universal "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/universalmessage"
)

func TestWrappedErrorClassification(t *testing.T) {
	possiblySucceeded := NewError("command outcome unknown", true, true)
	tests := []struct {
		name             string
		err              error
		mayHaveSucceeded bool
		temporary        bool
		shouldRetry      bool
	}{
		{
			name:        "temporary error",
			err:         fmt.Errorf("wrapped: %w", ErrBusy),
			temporary:   true,
			shouldRetry: true,
		},
		{
			name:             "possibly succeeded error",
			err:              fmt.Errorf("wrapped: %w", possiblySucceeded),
			mayHaveSucceeded: true,
			temporary:        true,
			shouldRetry:      false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := MayHaveSucceeded(test.err); got != test.mayHaveSucceeded {
				t.Errorf("MayHaveSucceeded() = %v, want %v", got, test.mayHaveSucceeded)
			}
			if got := Temporary(test.err); got != test.temporary {
				t.Errorf("Temporary() = %v, want %v", got, test.temporary)
			}
			if got := ShouldRetry(test.err); got != test.shouldRetry {
				t.Errorf("ShouldRetry() = %v, want %v", got, test.shouldRetry)
			}
		})
	}
}

func TestRoutableMessageErrorString(t *testing.T) {
	tests := []struct {
		name         string
		code         universal.MessageFault_E
		wantContains []string
		wantExact    string
	}{
		{
			name: "response MTU exceeded is descriptive",
			code: universal.MessageFault_E_MESSAGEFAULT_ERROR_RESPONSE_MTU_EXCEEDED,
			wantContains: []string{
				"MESSAGEFAULT_ERROR_RESPONSE_MTU_EXCEEDED",
				"received the request",
				"maximum message size",
			},
		},
		{
			name: "request MTU exceeded is descriptive",
			code: universal.MessageFault_E_MESSAGEFAULT_ERROR_REQUEST_MTU_EXCEEDED,
			wantContains: []string{
				"MESSAGEFAULT_ERROR_REQUEST_MTU_EXCEEDED",
				"not processed",
			},
		},
		{
			name:      "undocumented fault keeps bare enum name",
			code:      universal.MessageFault_E_MESSAGEFAULT_ERROR_BUSY,
			wantExact: "MESSAGEFAULT_ERROR_BUSY",
		},
		{
			name:      "unrecognized code",
			code:      universal.MessageFault_E(math.MaxInt32),
			wantExact: fmt.Sprintf("unrecognized error code %d", math.MaxInt32),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := (&RoutableMessageError{Code: test.code}).Error()
			if test.wantExact != "" && got != test.wantExact {
				t.Errorf("Error() = %q, want %q", got, test.wantExact)
			}
			for _, want := range test.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("Error() = %q, missing %q", got, want)
				}
			}
		})
	}
}

func TestMessageFaultDescriptionsCoverKnownCodes(t *testing.T) {
	for code := range messageFaultDescriptions {
		if _, ok := universal.MessageFault_E_name[int32(code)]; !ok {
			t.Errorf("description registered for unknown fault code %d", code)
		}
	}
}

// The vehicle drops the response, not the request, when it reports RESPONSE_MTU_EXCEEDED. Clients
// must therefore treat the command as possibly executed, and retrying blindly will not help because
// the vehicle will compose the same oversized reply.
func TestResponseMTUExceededClassification(t *testing.T) {
	err := fmt.Errorf("get drive state: %w", &RoutableMessageError{
		Code: universal.MessageFault_E_MESSAGEFAULT_ERROR_RESPONSE_MTU_EXCEEDED,
	})
	if !MayHaveSucceeded(err) {
		t.Error("MayHaveSucceeded() = false, want true")
	}
	if Temporary(err) {
		t.Error("Temporary() = true, want false")
	}
	if ShouldRetry(err) {
		t.Error("ShouldRetry() = true, want false")
	}
	var rmErr *RoutableMessageError
	if !errors.As(err, &rmErr) {
		t.Fatal("errors.As failed to recover *RoutableMessageError")
	}
	if rmErr.Code != universal.MessageFault_E_MESSAGEFAULT_ERROR_RESPONSE_MTU_EXCEEDED {
		t.Errorf("Code = %s, want RESPONSE_MTU_EXCEEDED", rmErr.Code)
	}
}

func TestRetriableError(t *testing.T) {
	var err RoutableMessageError
	var shouldRetry bool
	for code, message := range universal.MessageFault_E_name {
		switch universal.MessageFault_E(code) {
		case universal.MessageFault_E_MESSAGEFAULT_ERROR_NONE:
			shouldRetry = false
		case universal.MessageFault_E_MESSAGEFAULT_ERROR_BUSY:
			shouldRetry = true
		case universal.MessageFault_E_MESSAGEFAULT_ERROR_TIMEOUT:
			shouldRetry = true
		case universal.MessageFault_E_MESSAGEFAULT_ERROR_UNKNOWN_KEY_ID:
			shouldRetry = false
		case universal.MessageFault_E_MESSAGEFAULT_ERROR_INACTIVE_KEY:
			shouldRetry = false
		case universal.MessageFault_E_MESSAGEFAULT_ERROR_INVALID_SIGNATURE:
			shouldRetry = true
		case universal.MessageFault_E_MESSAGEFAULT_ERROR_INVALID_TOKEN_OR_COUNTER:
			shouldRetry = true
		case universal.MessageFault_E_MESSAGEFAULT_ERROR_INSUFFICIENT_PRIVILEGES:
			shouldRetry = false
		case universal.MessageFault_E_MESSAGEFAULT_ERROR_INVALID_DOMAINS:
			shouldRetry = false
		case universal.MessageFault_E_MESSAGEFAULT_ERROR_INVALID_COMMAND:
			shouldRetry = false
		case universal.MessageFault_E_MESSAGEFAULT_ERROR_DECODING:
			shouldRetry = false
		case universal.MessageFault_E_MESSAGEFAULT_ERROR_INTERNAL:
			shouldRetry = true
		case universal.MessageFault_E_MESSAGEFAULT_ERROR_WRONG_PERSONALIZATION:
			shouldRetry = false
		case universal.MessageFault_E_MESSAGEFAULT_ERROR_BAD_PARAMETER:
			shouldRetry = false
		case universal.MessageFault_E_MESSAGEFAULT_ERROR_KEYCHAIN_IS_FULL:
			shouldRetry = false
		case universal.MessageFault_E_MESSAGEFAULT_ERROR_INCORRECT_EPOCH:
			shouldRetry = true
		case universal.MessageFault_E_MESSAGEFAULT_ERROR_IV_INCORRECT_LENGTH:
			shouldRetry = false
		case universal.MessageFault_E_MESSAGEFAULT_ERROR_TIME_EXPIRED:
			shouldRetry = true
		case universal.MessageFault_E_MESSAGEFAULT_ERROR_NOT_PROVISIONED_WITH_IDENTITY:
			shouldRetry = false
		case universal.MessageFault_E_MESSAGEFAULT_ERROR_COULD_NOT_HASH_METADATA:
			shouldRetry = false
		case universal.MessageFault_E_MESSAGEFAULT_ERROR_TIME_TO_LIVE_TOO_LONG:
			shouldRetry = true
		case universal.MessageFault_E_MESSAGEFAULT_ERROR_REMOTE_ACCESS_DISABLED:
			shouldRetry = false
		case universal.MessageFault_E_MESSAGEFAULT_ERROR_REMOTE_SERVICE_ACCESS_DISABLED:
			shouldRetry = false
		case universal.MessageFault_E_MESSAGEFAULT_ERROR_COMMAND_REQUIRES_ACCOUNT_CREDENTIALS:
			shouldRetry = false
		case universal.MessageFault_E_MESSAGEFAULT_ERROR_REQUEST_MTU_EXCEEDED:
			shouldRetry = false
		case universal.MessageFault_E_MESSAGEFAULT_ERROR_RESPONSE_MTU_EXCEEDED:
			shouldRetry = false
		case universal.MessageFault_E_MESSAGEFAULT_ERROR_REPEATED_COUNTER:
			shouldRetry = false
		case universal.MessageFault_E_MESSAGEFAULT_ERROR_INVALID_KEY_HANDLE:
			shouldRetry = false
		case universal.MessageFault_E_MESSAGEFAULT_ERROR_REQUIRES_RESPONSE_ENCRYPTION:
			shouldRetry = false
		default:
			t.Fatalf("No expected retry behavior specified for %s", message)
		}
		err.Code = universal.MessageFault_E(code)
		if ShouldRetry(&err) != shouldRetry {
			t.Errorf("Unexpected retry behavior for error %s", message)
		}
	}
}

func TestErrKeyNameRequiresFleetAPI(t *testing.T) {
	t.Parallel()
	if Temporary(ErrKeyNameRequiresFleetAPI) || MayHaveSucceeded(ErrKeyNameRequiresFleetAPI) || ShouldRetry(ErrKeyNameRequiresFleetAPI) {
		t.Fatal("key-name Fleet requirement is permanent and must not retry")
	}
	if !errors.Is(fmt.Errorf("rename: %w", ErrKeyNameRequiresFleetAPI), ErrKeyNameRequiresFleetAPI) {
		t.Fatal("callers must be able to errors.Is the Fleet-only key name error")
	}
}

func TestErrWiFiNotInProtocol(t *testing.T) {
	t.Parallel()
	if Temporary(ErrWiFiNotInProtocol) || MayHaveSucceeded(ErrWiFiNotInProtocol) || ShouldRetry(ErrWiFiNotInProtocol) {
		t.Fatal("unpublished WiFi commands must not retry")
	}
	if !errors.Is(fmt.Errorf("add network: %w", ErrWiFiNotInProtocol), ErrWiFiNotInProtocol) {
		t.Fatal("callers must be able to errors.Is ErrWiFiNotInProtocol")
	}
}

func TestErrKeepAwakeNotInProtocol(t *testing.T) {
	t.Parallel()
	if Temporary(ErrKeepAwakeNotInProtocol) || MayHaveSucceeded(ErrKeepAwakeNotInProtocol) || ShouldRetry(ErrKeepAwakeNotInProtocol) {
		t.Fatal("unpublished keep-awake must not retry")
	}
	if !errors.Is(fmt.Errorf("keep awake: %w", ErrKeepAwakeNotInProtocol), ErrKeepAwakeNotInProtocol) {
		t.Fatal("callers must be able to errors.Is ErrKeepAwakeNotInProtocol")
	}
}

func TestErrBatteryOptionCatalog(t *testing.T) {
	t.Parallel()
	for _, err := range []error{ErrBatteryOptionRequiresFleetAPI, ErrBatteryOptionNotInCatalog} {
		if Temporary(err) || MayHaveSucceeded(err) || ShouldRetry(err) {
			t.Fatalf("%v must not retry", err)
		}
		if !errors.Is(fmt.Errorf("battery: %w", err), err) {
			t.Fatalf("callers must be able to errors.Is %v", err)
		}
	}
}

func TestErrBLEKeyPresenceNotInProtocol(t *testing.T) {
	t.Parallel()
	if Temporary(ErrBLEKeyPresenceNotInProtocol) || MayHaveSucceeded(ErrBLEKeyPresenceNotInProtocol) || ShouldRetry(ErrBLEKeyPresenceNotInProtocol) {
		t.Fatal("unpublished BLE presence-exempt keys must not retry")
	}
	if !errors.Is(fmt.Errorf("in-car button: %w", ErrBLEKeyPresenceNotInProtocol), ErrBLEKeyPresenceNotInProtocol) {
		t.Fatal("callers must be able to errors.Is ErrBLEKeyPresenceNotInProtocol")
	}
}

func TestErrScheduledChargingFirmware(t *testing.T) {
	t.Parallel()
	if Temporary(ErrScheduledChargingFirmware) || MayHaveSucceeded(ErrScheduledChargingFirmware) || ShouldRetry(ErrScheduledChargingFirmware) {
		t.Fatal("firmware scheduled-charging limitation must not retry")
	}
	if !errors.Is(fmt.Errorf("overheat: %w", ErrScheduledChargingFirmware), ErrScheduledChargingFirmware) {
		t.Fatal("callers must be able to errors.Is ErrScheduledChargingFirmware")
	}
}

func TestErrBLEStateLatencyFirmware(t *testing.T) {
	t.Parallel()
	if Temporary(ErrBLEStateLatencyFirmware) || MayHaveSucceeded(ErrBLEStateLatencyFirmware) || ShouldRetry(ErrBLEStateLatencyFirmware) {
		t.Fatal("firmware BLE GetDriveState latency floor must not retry")
	}
	if !errors.Is(fmt.Errorf("poll: %w", ErrBLEStateLatencyFirmware), ErrBLEStateLatencyFirmware) {
		t.Fatal("callers must be able to errors.Is ErrBLEStateLatencyFirmware")
	}
	if !strings.Contains(ErrBLEStateLatencyFirmware.Error(), "#414") {
		t.Fatal("error must cite teslamotors/vehicle-command#414")
	}
}

func TestErrSeatClimateFleetAPI(t *testing.T) {
	t.Parallel()
	if Temporary(ErrSeatClimateFleetAPI) || MayHaveSucceeded(ErrSeatClimateFleetAPI) || ShouldRetry(ErrSeatClimateFleetAPI) {
		t.Fatal("Tesla signed_command 501 on seat climate must not retry as a missing handler")
	}
	if !errors.Is(fmt.Errorf("501: %w", ErrSeatClimateFleetAPI), ErrSeatClimateFleetAPI) {
		t.Fatal("callers must be able to errors.Is ErrSeatClimateFleetAPI")
	}
	if !strings.Contains(ErrSeatClimateFleetAPI.Error(), "#383") {
		t.Fatal("error must cite teslamotors/vehicle-command#383")
	}
	if !strings.Contains(ErrSeatClimateFleetAPI.Error(), "are implemented") {
		t.Fatal("error must state seat heater/cooler VehicleActions are implemented")
	}
}

func TestErrPartnerOAuthNotProvisioned(t *testing.T) {
	t.Parallel()
	if Temporary(ErrPartnerOAuthNotProvisioned) || MayHaveSucceeded(ErrPartnerOAuthNotProvisioned) || ShouldRetry(ErrPartnerOAuthNotProvisioned) {
		t.Fatal("Tesla OAuth audience/policy provisioning must not retry as a client audience typo")
	}
	if !errors.Is(fmt.Errorf("token: %w", ErrPartnerOAuthNotProvisioned), ErrPartnerOAuthNotProvisioned) {
		t.Fatal("callers must be able to errors.Is ErrPartnerOAuthNotProvisioned")
	}
	if !strings.Contains(ErrPartnerOAuthNotProvisioned.Error(), "#460") {
		t.Fatal("error must cite teslamotors/vehicle-command#460")
	}
	if !strings.Contains(ErrPartnerOAuthNotProvisioned.Error(), "invalid_audience") {
		t.Fatal("error must name Tesla Fleet Auth invalid_audience")
	}
}

func TestErrChargingManagerChargePortFirmware(t *testing.T) {
	t.Parallel()
	if Temporary(ErrChargingManagerChargePortFirmware) || MayHaveSucceeded(ErrChargingManagerChargePortFirmware) || ShouldRetry(ErrChargingManagerChargePortFirmware) {
		t.Fatal("firmware Charging Manager charge-port ACL must not retry")
	}
	if !errors.Is(fmt.Errorf("charge door: %w", ErrChargingManagerChargePortFirmware), ErrChargingManagerChargePortFirmware) {
		t.Fatal("callers must be able to errors.Is ErrChargingManagerChargePortFirmware")
	}
	if !strings.Contains(ErrChargingManagerChargePortFirmware.Error(), "#413") {
		t.Fatal("error must cite teslamotors/vehicle-command#413")
	}
}

func TestErrHvacAutoModeNotInProtocol(t *testing.T) {
	t.Parallel()
	if Temporary(ErrHvacAutoModeNotInProtocol) || MayHaveSucceeded(ErrHvacAutoModeNotInProtocol) || ShouldRetry(ErrHvacAutoModeNotInProtocol) {
		t.Fatal("unpublished HVAC Auto vs Manual must not retry")
	}
	if !errors.Is(fmt.Errorf("hvac auto: %w", ErrHvacAutoModeNotInProtocol), ErrHvacAutoModeNotInProtocol) {
		t.Fatal("callers must be able to errors.Is ErrHvacAutoModeNotInProtocol")
	}
	if !strings.Contains(ErrHvacAutoModeNotInProtocol.Error(), "#283") {
		t.Fatal("error must cite teslamotors/vehicle-command#283")
	}
}

func TestErrClimateSplitNotInProtocol(t *testing.T) {
	t.Parallel()
	if Temporary(ErrClimateSplitNotInProtocol) || MayHaveSucceeded(ErrClimateSplitNotInProtocol) || ShouldRetry(ErrClimateSplitNotInProtocol) {
		t.Fatal("unpublished climate split/SYNC must not retry")
	}
	if !errors.Is(fmt.Errorf("sync: %w", ErrClimateSplitNotInProtocol), ErrClimateSplitNotInProtocol) {
		t.Fatal("callers must be able to errors.Is ErrClimateSplitNotInProtocol")
	}
	if !strings.Contains(ErrClimateSplitNotInProtocol.Error(), "#386") {
		t.Fatal("error must cite teslamotors/vehicle-command#386")
	}
}

func TestErrChargingWhileInfotainmentAsleep(t *testing.T) {
	t.Parallel()
	if Temporary(ErrChargingWhileInfotainmentAsleep) || MayHaveSucceeded(ErrChargingWhileInfotainmentAsleep) || ShouldRetry(ErrChargingWhileInfotainmentAsleep) {
		t.Fatal("telemetry-vs-Infotainment charging must not retry in Vehicle.Send")
	}
	if !errors.Is(fmt.Errorf("charge stop: %w", ErrChargingWhileInfotainmentAsleep), ErrChargingWhileInfotainmentAsleep) {
		t.Fatal("callers must be able to errors.Is ErrChargingWhileInfotainmentAsleep")
	}
	if !strings.Contains(ErrChargingWhileInfotainmentAsleep.Error(), "#452") {
		t.Fatal("error must cite teslamotors/vehicle-command#452")
	}
}

func TestErrInfotainmentSignedCommandOffline(t *testing.T) {
	t.Parallel()
	if Temporary(ErrInfotainmentSignedCommandOffline) || MayHaveSucceeded(ErrInfotainmentSignedCommandOffline) || ShouldRetry(ErrInfotainmentSignedCommandOffline) {
		t.Fatal("Infotainment signed_command 408 must not retry in Vehicle.Send")
	}
	if !errors.Is(fmt.Errorf("session: %w", ErrInfotainmentSignedCommandOffline), ErrInfotainmentSignedCommandOffline) {
		t.Fatal("callers must be able to errors.Is ErrInfotainmentSignedCommandOffline")
	}
	if !strings.Contains(ErrInfotainmentSignedCommandOffline.Error(), "#285") {
		t.Fatal("error must cite teslamotors/vehicle-command#285")
	}
}

func TestErrClimateKeeperCPDFirmware(t *testing.T) {
	t.Parallel()
	if Temporary(ErrClimateKeeperCPDFirmware) || MayHaveSucceeded(ErrClimateKeeperCPDFirmware) || ShouldRetry(ErrClimateKeeperCPDFirmware) {
		t.Fatal("CPD Dog/Camp refusal must not retry in Vehicle.Send")
	}
	if !errors.Is(fmt.Errorf("dog mode: %w", ErrClimateKeeperCPDFirmware), ErrClimateKeeperCPDFirmware) {
		t.Fatal("callers must be able to errors.Is ErrClimateKeeperCPDFirmware")
	}
	if !strings.Contains(ErrClimateKeeperCPDFirmware.Error(), "#509") {
		t.Fatal("error must cite teslamotors/vehicle-command#509")
	}
}

func TestErrVirtualKeyReturnURI(t *testing.T) {
	t.Parallel()
	if Temporary(ErrVirtualKeyReturnURI) || MayHaveSucceeded(ErrVirtualKeyReturnURI) || ShouldRetry(ErrVirtualKeyReturnURI) {
		t.Fatal("virtual key return_uri must not retry in Vehicle.Send")
	}
	if !errors.Is(fmt.Errorf("finish setup: %w", ErrVirtualKeyReturnURI), ErrVirtualKeyReturnURI) {
		t.Fatal("callers must be able to errors.Is ErrVirtualKeyReturnURI")
	}
	if !strings.Contains(ErrVirtualKeyReturnURI.Error(), "#444") {
		t.Fatal("error must cite teslamotors/vehicle-command#444")
	}
}

func TestIsClimateKeeperCPDEnabled(t *testing.T) {
	t.Parallel()
	nominal := &NominalError{Details: NewError("car could not execute command: cpd_enabled", false, false)}
	if !IsClimateKeeperCPDEnabled(nominal) {
		t.Fatal("NominalError cpd_enabled must match IsClimateKeeperCPDEnabled")
	}
	if !IsClimateKeeperCPDEnabled(fmt.Errorf("proxy: %w", nominal)) {
		t.Fatal("wrapped NominalError cpd_enabled must match")
	}
	if !IsClimateKeeperCPDEnabled(ErrClimateKeeperCPDFirmware) {
		t.Fatal("ErrClimateKeeperCPDFirmware must match IsClimateKeeperCPDEnabled")
	}
	other := &NominalError{Details: NewError("car could not execute command: already_on", false, false)}
	if IsClimateKeeperCPDEnabled(other) {
		t.Fatal("unrelated NominalError must not match")
	}
	if IsClimateKeeperCPDEnabled(nil) || IsClimateKeeperCPDEnabled(errors.New("cpd_enabled")) {
		t.Fatal("plain errors and nil must not match")
	}
}

func TestErrBoomboxNotInProtocol(t *testing.T) {
	t.Parallel()
	if Temporary(ErrBoomboxNotInProtocol) || MayHaveSucceeded(ErrBoomboxNotInProtocol) || ShouldRetry(ErrBoomboxNotInProtocol) {
		t.Fatal("unpublished boombox must not retry")
	}
	if !errors.Is(fmt.Errorf("fart: %w", ErrBoomboxNotInProtocol), ErrBoomboxNotInProtocol) {
		t.Fatal("callers must be able to errors.Is ErrBoomboxNotInProtocol")
	}
	if !strings.Contains(ErrBoomboxNotInProtocol.Error(), "#266") {
		t.Fatal("error must cite teslamotors/vehicle-command#266 legal hold")
	}
}
