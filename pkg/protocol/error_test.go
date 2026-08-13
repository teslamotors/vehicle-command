package protocol

import (
	"fmt"
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
