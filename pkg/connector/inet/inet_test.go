package inet

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

func TestVehicleNotAwakeFromTeslaGateway(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{"service unavailable", http.StatusServiceUnavailable, `{"error":"vehicle unavailable: vehicle is offline or asleep"}`},
		{"request timeout offline", http.StatusRequestTimeout, `{"error":"vehicle is offline"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()
			_, err := SendFleetAPICommand(context.Background(), server.Client(), "test", "Bearer x", server.URL+"/api/1/vehicles/VIN/signed_command", []byte("{}"))
			if !errors.Is(err, ErrVehicleNotAwake) {
				t.Fatalf("err = %v, want ErrVehicleNotAwake", err)
			}
			if protocol.ShouldRetry(err) || protocol.Temporary(err) {
				t.Fatal("ErrVehicleNotAwake must not trigger Vehicle.Send retries; callers wake then retry at the application layer")
			}
		})
	}
}
