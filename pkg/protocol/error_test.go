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
