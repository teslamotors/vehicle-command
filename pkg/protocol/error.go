package protocol

import (
	"errors"
	"fmt"

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
	ErrRequiresEncryption   = errors.New("command should not be sent in plaintext or encrypted with an unauthenticated public key")
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
	if commErr, ok := err.(Error); ok && commErr.MayHaveSucceeded() {
		return true
	}
	return false
}

// Temporary returns true if err is a CommandError that indicates the command failed due to possibly
// transient conditions that do not require user action to resolve.
func Temporary(err error) bool {
	if commErr, ok := err.(Error); ok && commErr.Temporary() {
		return true
	}
	return false
}

// ShouldRetry returns true if the client should retry to issue the command that triggered an error.
func ShouldRetry(err error) bool {
	if err == nil {
		return false
	}
	if e, ok := err.(Error); ok {
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
type RoutableMessageError struct {
	Code universal.MessageFault_E
}

func (v *RoutableMessageError) MayHaveSucceeded() bool {
	return v.Code == universal.MessageFault_E_MESSAGEFAULT_ERROR_NONE
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
	if errString, ok := universal.MessageFault_E_name[int32(v.Code)]; ok {
		return errString
	}
	return fmt.Sprintf("unrecognized error code %d", v.Code)
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
