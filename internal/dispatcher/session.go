package dispatcher

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/teslamotors/vehicle-command/internal/authentication"
	"github.com/teslamotors/vehicle-command/pkg/connector"

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
}

// NewSession creates a new session object that can authorize commands going to
// the vehicle and authenticate session info arriving from the vehicle.
func NewSession(private authentication.ECDHPrivateKey, vin string) (*session, error) {
	return &session{
		private:     private,
		readySignal: make(chan struct{}, 1),
		vin:         []byte(vin),
	}, nil
}

func (s *session) Authorize(ctx context.Context, command *universal.RoutableMessage, method connector.AuthMethod) error {
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

// VehiclePublicKeyBytes returns the encoded remote public key.
func (s *session) VehiclePublicKeyBytes() []byte {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.ctx == nil {
		return nil
	}
	return s.ctx.RemotePublicKeyBytes()
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

// ProcessHello verifies a session info message from the vehicle.
//
// The caller must verify that the challenge matches the UUID of a
// recently-transmitted message.
func (s *session) ProcessHello(challenge, info, tag []byte) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	var err error
	if s.ctx == nil {
		s.ctx, err = authentication.NewAuthenticatedSigner(s.private, s.vin, challenge, info, tag)
		if err != nil {
			return err
		}
	} else {
		err = s.ctx.UpdateSignedSessionInfo(challenge, info, tag)
	}

	if err == nil && !s.ready {
		s.ready = true
		close(s.readySignal) // Notifies blocked goroutines that we're ready to authorize commands
	}
	return err
}
