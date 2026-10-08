package dispatcher

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/teslamotors/vehicle-command/pkg/protocol"
	universal "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/universalmessage"
)

// startSessionsWorkerFrame identifies the worker goroutines that StartSessions
// spawns to run parallel handshakes in goroutine stack dumps.
const startSessionsWorkerFrame = "StartSessions.func1"

// countStartSessionsWorkers returns the number of live goroutines that are
// currently executing a StartSessions worker function.
func countStartSessionsWorkers() int {
	buf := make([]byte, 1<<20)
	n := runtime.Stack(buf, true)
	count := 0
	for _, goroutine := range strings.Split(string(buf[:n]), "\ngoroutine ") {
		if strings.Contains(goroutine, startSessionsWorkerFrame) {
			count++
		}
	}
	return count
}

// waitForStartSessionsWorkers polls until no StartSessions worker goroutines
// remain, returning the number still alive after the timeout expires.
func waitForStartSessionsWorkers(timeout time.Duration) int {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if n := countStartSessionsWorkers(); n == 0 {
			return 0
		}
		time.Sleep(10 * time.Millisecond)
	}
	return countStartSessionsWorkers()
}

// TestStartSessionsEarlyHandshakeError verifies that StartSessions does not leak
// worker goroutines when a handshake fails immediately with a non-cancellation
// error (regression test for issue #494).
//
// With a nil private key, every StartSession fails immediately with
// protocol.ErrRequiresKey. StartSessions returns after receiving the first
// error, so any worker that is still alive after the call has returned is
// permanently blocked reporting its result through the results channel.
func TestStartSessionsEarlyHandshakeError(t *testing.T) {
	conn := newDummyConnector(t)
	d, err := New(conn, nil)
	if err != nil {
		t.Fatalf("New() failed: %s", err)
	}

	domains := []universal.Domain{
		universal.Domain_DOMAIN_VEHICLE_SECURITY,
		universal.Domain_DOMAIN_INFOTAINMENT,
	}
	err = d.StartSessions(context.Background(), domains)
	if !errors.Is(err, protocol.ErrRequiresKey) {
		t.Fatalf("StartSessions() returned %v, expected protocol.ErrRequiresKey", err)
	}

	// Give the spawned workers a chance to run. (Immediately after
	// StartSessions returns, the worker reporting the second result may not
	// have been scheduled yet, so a zero count at this point proves nothing.)
	deadline := time.Now().Add(2 * time.Second)
	for countStartSessionsWorkers() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	if leaked := waitForStartSessionsWorkers(10 * time.Second); leaked > 0 {
		t.Errorf("StartSessions left %d worker goroutine(s) running", leaked)
	}
}
