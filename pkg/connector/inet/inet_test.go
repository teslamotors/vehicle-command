package inet

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/teslamotors/vehicle-command/pkg/protocol"
)

func TestSendAfterClose(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"response": ""}`))
	}))
	defer server.Close()
	domain, _ := strings.CutPrefix(server.URL, "https://")
	conn := NewConnection("VIN123", "", domain, "")
	conn.client = server.Client()
	if err := conn.Send(context.Background(), []byte{}); err != nil {
		t.Errorf("Send failed: %s", err)
	}
	conn.Close()
	if err := conn.Send(context.Background(), []byte{}); err != protocol.ErrNotConnected {
		t.Errorf("Expected ErrNotConnected but got %s", err)
	}
}

// wakeupServer is a fake Fleet API. wakeReplies are the HTTP status codes for successive wake_up
// requests (200 replies report "asleep"), and pollStates are the states reported by successive
// vehicle endpoint requests. The last entry of each repeats.
type wakeupServer struct {
	wakeReplies []int
	pollStates  []string
	wakes       int
	polls       int
}

func pick[T any](values []T, i int) T {
	return values[min(i, len(values)-1)]
}

func newWakeupTestConnection(t *testing.T, s *wakeupServer) *Connection {
	t.Helper()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/1/vehicles/VIN123/wake_up":
			code := pick(s.wakeReplies, s.wakes)
			s.wakes++
			w.WriteHeader(code)
			if code == http.StatusOK {
				fmt.Fprint(w, `{"response": {"state": "asleep"}}`)
			}
		case r.Method == http.MethodGet && r.URL.Path == "/api/1/vehicles/VIN123":
			fmt.Fprintf(w, `{"response": {"state": %q}}`, pick(s.pollStates, s.polls))
			s.polls++
		default:
			t.Errorf("Unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	domain, _ := strings.CutPrefix(server.URL, "https://")
	conn := NewConnection("VIN123", "", domain, "")
	conn.client = server.Client()

	interval := wakeupPollInterval
	wakeupPollInterval = time.Millisecond
	t.Cleanup(func() { wakeupPollInterval = interval })
	return conn
}

func TestWakeupPollsVehicleStateUntilOnline(t *testing.T) {
	s := &wakeupServer{wakeReplies: []int{http.StatusOK}, pollStates: []string{"asleep", "asleep", "online"}}
	conn := newWakeupTestConnection(t, s)
	if err := conn.Wakeup(context.Background()); err != nil {
		t.Fatalf("Wakeup failed: %s", err)
	}
	if s.wakes != 1 || s.polls != 3 {
		t.Errorf("Expected 1 wake_up and 3 state requests but got %d and %d", s.wakes, s.polls)
	}
}

func TestWakeupResendsFailedWakeRequest(t *testing.T) {
	s := &wakeupServer{wakeReplies: []int{http.StatusGatewayTimeout, http.StatusOK}, pollStates: []string{"online"}}
	conn := newWakeupTestConnection(t, s)
	if err := conn.Wakeup(context.Background()); err != nil {
		t.Fatalf("Wakeup failed: %s", err)
	}
	if s.wakes != 2 || s.polls != 1 {
		t.Errorf("Expected 2 wake_up and 1 state requests but got %d and %d", s.wakes, s.polls)
	}
}

func TestWakeupPendingWhenContextExpires(t *testing.T) {
	s := &wakeupServer{wakeReplies: []int{http.StatusOK}, pollStates: []string{"asleep"}}
	conn := newWakeupTestConnection(t, s)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := conn.Wakeup(ctx)
	if !errors.Is(err, ErrWakeupPending) || !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected ErrWakeupPending and context.DeadlineExceeded but got %v", err)
	}
	if s.wakes != 1 {
		t.Errorf("Expected 1 wake_up request but got %d", s.wakes)
	}
}
