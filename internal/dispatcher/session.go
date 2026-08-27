package dispatcher

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/teslamotors/vehicle-command/internal/authentication"
	"github.com/teslamotors/vehicle-command/pkg/connector"
	"github.com/teslamotors/vehicle-command/pkg/protocol"

	universal "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/universalmessage"
)

var defaultExpiration = 5 * time.Second

// CacheEntry contains information that allows a vehicle session to be resumed without a handshake
// mesasge (SessionInfoRequest).
type CacheEntry struct {
	CreatedAt   time.Time `json:"created_at"`
	Domain      int       `json:"domain"`
	SessionInfo []byte    `json:"data"`
}

type session struct {
	// Goroutines may hold the lock at times when they should be responsive to
	// a context.Context object being cancelled; therefore they should never
	// hold the lock during potentially long-running operations.
	lock        sync.Mutex
	ctx         *authentication.Signer
	vin         []byte
	private     authentication.ECDHPrivateKey
	ready       bool
	readySignal chan struct{}
	// verificationErr records the most recent failure to verify session info
	// received from the vehicle (for example, an invalid session-info HMAC
	// because the vehicle's key changed after a service visit). It is cleared
	// whenever session info verifies successfully. A non-nil value on an
	// otherwise-ready session means the session can no longer authenticate the
	// vehicle and must be re-established with a fresh handshake.
	verificationErr error
}

// newSession creates a new session object that can authorize commands going to
// the vehicle and authenticate session info arriving from the vehicle.
func newSession(private authentication.ECDHPrivateKey, vin string) (*session, error) {
	return &session{
		private:     private,
		readySignal: make(chan struct{}, 1),
		vin:         []byte(vin),
	}, nil
}

func (s *session) decrypt(message *universal.RoutableMessage, handler *receiver) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	counter, err := s.ctx.Decrypt(message, handler.requestID)
	if err != nil {
		return err
	}
	if !handler.antireplay.Update(counter) {
		return protocol.ErrReplayedResponse
	}
	return nil
}

func (s *session) authorize(ctx context.Context, command *universal.RoutableMessage, method connector.AuthMethod) error {
	var err error
	lifetime := defaultExpiration
	if deadline, ok := ctx.Deadline(); ok {
		lifetime = time.Until(deadline)
	}
	for {
		attempted := false
		select {
		case <-s.readySignal:
			// Prevent a race condition where the goroutine may unblock but the
			// session becomes invalid before it authorizes the command.
			s.lock.Lock()
			if s.verificationErr != nil {
				// The vehicle's session info can no longer be verified with this
				// session's keys (for example, the vehicle's key changed). The
				// session will never recover on its own, so fail terminally with
				// the underlying error instead of retrying until the context
				// deadline. StartSession discards the session and performs a
				// fresh handshake on its next call.
				err = s.verificationErr
				s.lock.Unlock()
				return err
			}
			if s.ctx != nil && s.ready {
				switch method {
				case connector.AuthMethodNone:
					err = nil
				case connector.AuthMethodGCM:
					err = s.ctx.Encrypt(command, lifetime)
				case connector.AuthMethodHMAC:
					err = s.ctx.AuthorizeHMAC(command, lifetime)
				default:
					return errors.New("unrecognized authentication method")
				}
				attempted = true
			}
			s.lock.Unlock()
			if err != nil {
				// Retry until caller cancels context
				err = nil
			} else if attempted {
				return nil
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (s *session) export() []byte {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.ctx == nil {
		return nil
	}
	info, err := s.ctx.ExportSessionInfo()
	if err != nil {
		return nil
	}
	return info
}

// processHello verifies a session info message from the vehicle.
//
// The caller must verify that the challenge matches the UUID of a
// recently-transmitted message.
func (s *session) processHello(challenge, info, tag []byte) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	var err error
	if s.ctx == nil {
		s.ctx, err = authentication.NewAuthenticatedSigner(s.private, s.vin, challenge, info, tag)
	} else {
		err = s.ctx.UpdateSignedSessionInfo(challenge, info, tag)
	}

	if err != nil {
		// Record the failure so that a session that can no longer verify the
		// vehicle's session info fails commands terminally instead of retrying
		// forever, and so StartSession can discard and re-establish it.
		s.verificationErr = err
		return err
	}
	// Session info verified successfully, so any previously recorded fault
	// (such as a transient bad frame) is stale and must be cleared. This also
	// preserves the intended recovery path in which the vehicle proactively
	// includes fresh, correctly-signed session info to resync a desync.
	s.verificationErr = nil

	if !s.ready {
		s.ready = true
		close(s.readySignal) // Notifies blocked goroutines that we're ready to authorize commands
	}
	return nil
}

// verificationError returns the most recent session-info verification failure,
// or nil if the last verification succeeded. Callers must not hold s.lock.
func (s *session) verificationError() error {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.verificationErr
}
