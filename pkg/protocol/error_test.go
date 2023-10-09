package protocol

import (
	"testing"

	universal "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/universalmessage"
)

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
		default:
			t.Fatalf("No expected retry behavior specified for %s", message)
		}
		err.Code = universal.MessageFault_E(code)
		if ShouldRetry(&err) != shouldRetry {
			t.Errorf("Unexpected retry behavior for error %s", message)
		}
	}
}
