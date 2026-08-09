package inet

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/teslamotors/vehicle-command/pkg/protocol"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

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

func TestSetAuthHeaderFuncUsedPerRequest(t *testing.T) {
	var auths []string
	conn := NewConnection("VIN123", "Bearer static", "example.tesla.com", "ua")
	conn.client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			auths = append(auths, req.Header.Get("Authorization"))
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
				Header:     make(http.Header),
			}, nil
		}),
	}
	n := 0
	conn.SetAuthHeaderFunc(func() (string, error) {
		n++
		return fmt.Sprintf("Bearer tok-%d", n), nil
	})

	if _, err := conn.SendFleetAPICommand(context.Background(), "api/1/vehicles/VIN123/wake_up", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.SendFleetAPICommand(context.Background(), "api/1/vehicles/VIN123/wake_up", nil); err != nil {
		t.Fatal(err)
	}
	if len(auths) != 2 || auths[0] != "Bearer tok-1" || auths[1] != "Bearer tok-2" {
		t.Fatalf("auths=%v", auths)
	}
}
