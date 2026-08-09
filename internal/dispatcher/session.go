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
	// helloError is set when the most recent session-info (hello) verification
	// failed. Callers use it to surface crypto failures instead of waiting for
	// a context deadline after a stale cached session can no longer be updated.
	helloError error
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
		s.lock.Lock()
		if err := s.helloError; err != nil && !s.ready {
			s.lock.Unlock()
			return err
		}
		readySignal := s.readySignal
		s.lock.Unlock()

		attempted := false
		select {
		case <-readySignal:
			// Prevent a race condition where the goroutine may unblock but the
			// session becomes invalid before it authorizes the command.
			s.lock.Lock()
			if s.ctx != nil && s.ready {
				switch method {
				case connector.AuthMethodNone:
					err = nil
				case connector.AuthMethodGCM:
					err = s.ctx.Encrypt(command, lifetime)
				case connector.AuthMethodHMAC:
					err = s.ctx.AuthorizeHMAC(command, lifetime)
				default:
					s.lock.Unlock()
					return errors.New("unrecognized authentication method")
				}
				attempted = true
			} else if err := s.helloError; err != nil {
				s.lock.Unlock()
				return err
			}
			s.lock.Unlock()
			if err != nil {
				// Retry until caller cancels context
				err = nil
			} else if attempted {
				return nil
			}
		case <-ctx.Done():
			if err := s.lastHelloError(); err != nil {
				return err
			}
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
//
// If verification fails for an established session (for example after a
// vehicle security controller is replaced and the domain public key changes),
// the local session is invalidated so the next handshake can trust-on-first-use
// the new vehicle key. The verification error is retained for callers that
// would otherwise block until their context deadline.
func (s *session) processHello(challenge, info, tag []byte) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	var err error
	if s.ctx == nil {
		s.ctx, err = authentication.NewAuthenticatedSigner(s.private, s.vin, challenge, info, tag)
		if err != nil {
			s.helloError = err
			return err
		}
	} else {
		err = s.ctx.UpdateSignedSessionInfo(challenge, info, tag)
		if err != nil {
			s.invalidateLocked(err)
			return err
		}
	}

	s.helloError = nil
	if !s.ready {
		s.ready = true
		close(s.readySignal) // Notifies blocked goroutines that we're ready to authorize commands
	}
	return nil
}

// invalidateLocked drops a locally cached session so a fresh handshake is required.
// Caller must hold s.lock.
func (s *session) invalidateLocked(err error) {
	s.ctx = nil
	s.helloError = err
	if s.ready {
		s.ready = false
		// readySignal was already closed when the session became ready; allocate a
		// new channel for the next handshake.
		s.readySignal = make(chan struct{}, 1)
	}
}

func (s *session) lastHelloError() error {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.helloError
}

func (s *session) clearHelloError() {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.helloError = nil
}
