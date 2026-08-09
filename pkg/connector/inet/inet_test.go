package inet

import (
	"context"
	"io"
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
	conn.SetHTTPClient(server.Client())
	if err := conn.Send(context.Background(), []byte{}); err != nil {
		t.Errorf("Send failed: %s", err)
	}
	conn.Close()
	if err := conn.Send(context.Background(), []byte{}); err != protocol.ErrNotConnected {
		t.Errorf("Expected ErrNotConnected but got %s", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestSetHTTPClientUsedForFleetAPICommand(t *testing.T) {
	conn := NewConnection("VIN123", "Bearer tok", "fleet-api.prd.na.vn.cloud.tesla.com", "ua")
	var sawPath string
	conn.SetHTTPClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			sawPath = req.URL.Path
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
				Header:     make(http.Header),
			}, nil
		}),
	})

	if _, err := conn.SendFleetAPICommand(context.Background(), "api/1/vehicles/VIN123/wake_up", nil); err != nil {
		t.Fatalf("SendFleetAPICommand: %v", err)
	}
	if sawPath != "/api/1/vehicles/VIN123/wake_up" {
		t.Fatalf("path = %q, want wake_up endpoint", sawPath)
	}
}
