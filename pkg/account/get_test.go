package account

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/teslamotors/vehicle-command/pkg/connector"
)

func TestGetResponseLength(t *testing.T) {
	for _, chunked := range []bool{false, true} {
		for _, size := range []int{0, connector.MaxResponseLength - 1, connector.MaxResponseLength, connector.MaxResponseLength + 1, 2 * connector.MaxResponseLength} {
			t.Run(fmt.Sprintf("size=%d/chunked=%t", size, chunked), func(t *testing.T) {
				payload := strings.Repeat("x", size)
				server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					if chunked {
						w.(http.Flusher).Flush()
					} else {
						w.Header().Set("Content-Length", fmt.Sprint(size))
					}
					_, _ = w.Write([]byte(payload))
				}))
				defer server.Close()
				account := Account{Host: strings.TrimPrefix(server.URL, "https://"), client: *server.Client()}

				body, err := account.Get(context.Background(), "api/1/vehicles")
				if size > connector.MaxResponseLength {
					if err == nil {
						t.Fatalf("oversized response reported success with %d bytes", len(body))
					}
					if body != nil {
						t.Fatalf("oversized response returned a partial body of %d bytes", len(body))
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if string(body) != payload {
					t.Fatalf("response body differs: got %d bytes, want %d", len(body), size)
				}
			})
		}
	}
}
