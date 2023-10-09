package authentication

import (
	"fmt"
	"unicode"

	universal "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/universalmessage"
)

const (
	errCodeOk               = universal.MessageFault_E_MESSAGEFAULT_ERROR_NONE
	errCodeBusy             = universal.MessageFault_E_MESSAGEFAULT_ERROR_BUSY
	errCodeTimeout          = universal.MessageFault_E_MESSAGEFAULT_ERROR_TIMEOUT
	errCodeUnknownKey       = universal.MessageFault_E_MESSAGEFAULT_ERROR_UNKNOWN_KEY_ID
	errCodeInactiveKey      = universal.MessageFault_E_MESSAGEFAULT_ERROR_INACTIVE_KEY
	errCodeInvalidSignature = universal.MessageFault_E_MESSAGEFAULT_ERROR_INVALID_SIGNATURE
	errCodeInvalidToken     = universal.MessageFault_E_MESSAGEFAULT_ERROR_INVALID_TOKEN_OR_COUNTER
	errCodeForbidden        = universal.MessageFault_E_MESSAGEFAULT_ERROR_INSUFFICIENT_PRIVILEGES
	errCodeInvalidDomain    = universal.MessageFault_E_MESSAGEFAULT_ERROR_INVALID_DOMAINS
	errCodeInvalidCommand   = universal.MessageFault_E_MESSAGEFAULT_ERROR_INVALID_COMMAND
	errCodeDecoding         = universal.MessageFault_E_MESSAGEFAULT_ERROR_DECODING
	errCodeInternal         = universal.MessageFault_E_MESSAGEFAULT_ERROR_INTERNAL
	errCodeWrongPerso       = universal.MessageFault_E_MESSAGEFAULT_ERROR_WRONG_PERSONALIZATION
	errCodeBadParameter     = universal.MessageFault_E_MESSAGEFAULT_ERROR_BAD_PARAMETER
	errCodeKeychainFull     = universal.MessageFault_E_MESSAGEFAULT_ERROR_KEYCHAIN_IS_FULL
	errCodeExpired          = universal.MessageFault_E_MESSAGEFAULT_ERROR_TIME_EXPIRED
	errCodeInvalidEpoch     = universal.MessageFault_E_MESSAGEFAULT_ERROR_INCORRECT_EPOCH
)

// errCodeString returns a CamelCase error string for code.
func errCodeString(code universal.MessageFault_E) string {
	// "MESSAGEFAULT_ERROR_INCORRECT_EPOCH" -> "IncorrectEpoch"
	const prefix = "MESSAGEFAULT_ERROR_"
	allCaps := code.String()[len(prefix)-1:]
	camelCase := make([]rune, 0, len(allCaps))
	lowerCaseNext := false
	for _, b := range allCaps {
		if b == '_' {
			lowerCaseNext = false
		} else {
			if lowerCaseNext {
				camelCase = append(camelCase, unicode.ToLower(b))
			} else {
				camelCase = append(camelCase, b)
				lowerCaseNext = true
			}
		}

	}
	return string(camelCase)
}

// Error represents a protocol-layer error.
type Error struct {
	Code universal.MessageFault_E
	Info string
}

func newError(code universal.MessageFault_E, info string) error {
	return &Error{code, info}
}

func (e Error) Error() string {
	if e.Info == "" {
		return errCodeString(e.Code)
	}
	return fmt.Sprintf("%s: %s", errCodeString(e.Code), e.Info)
}
