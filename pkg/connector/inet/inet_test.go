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

func newWakeupTestConnection(t *testing.T, states ...string) (*Connection, *int) {
	t.Helper()
	requests := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		state := states[len(states)-1]
		if requests < len(states) {
			state = states[requests]
		}
		requests++
		fmt.Fprintf(w, `{"response": {"state": %q}}`, state)
	}))
	t.Cleanup(server.Close)
	domain, _ := strings.CutPrefix(server.URL, "https://")
	conn := NewConnection("VIN123", "", domain, "")
	conn.client = server.Client()

	interval := wakeupPollInterval
	wakeupPollInterval = time.Millisecond
	t.Cleanup(func() { wakeupPollInterval = interval })
	return conn, &requests
}

func TestWakeupPollsUntilOnline(t *testing.T) {
	conn, requests := newWakeupTestConnection(t, "asleep", "asleep", "online")
	if err := conn.Wakeup(context.Background()); err != nil {
		t.Fatalf("Wakeup failed: %s", err)
	}
	if *requests != 3 {
		t.Errorf("Expected 3 wake_up requests but got %d", *requests)
	}
}

func TestWakeupStopsWhenContextExpires(t *testing.T) {
	conn, _ := newWakeupTestConnection(t, "asleep")
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := conn.Wakeup(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected context.DeadlineExceeded but got %v", err)
	}
}
