package account

import (
	"errors"
	"strings"
	"testing"

	"github.com/teslamotors/vehicle-command/pkg/protocol"
)

func TestVirtualKeyInstallURL(t *testing.T) {
	t.Parallel()
	got, err := VirtualKeyInstallURL("Example.COM")
	if err != nil {
		t.Fatalf("VirtualKeyInstallURL: %v", err)
	}
	if got != "https://tesla.com/_ak/example.com" {
		t.Fatalf("url = %q", got)
	}
	if strings.Contains(got, "?") || strings.Contains(got, "return_uri") {
		t.Fatal("install link must not include return_uri")
	}
	for _, bad := range []string{
		"",
		"https://example.com",
		"example.com/finish-setup",
		"example.com:443",
		"user@example.com",
		"192.168.1.1",
		"localhost",
	} {
		if _, err := VirtualKeyInstallURL(bad); !errors.Is(err, protocol.ErrVirtualKeyReturnURI) {
			t.Errorf("VirtualKeyInstallURL(%q) err = %v, want ErrVirtualKeyReturnURI", bad, err)
		}
	}
}

func TestVirtualKeyReturnHostAllowed(t *testing.T) {
	t.Parallel()
	allowed := []string{
		"https://example.com/finish-setup",
		"https://app.example.com/done",
		"https://EXAMPLE.com/finish-setup",
	}
	for _, uri := range allowed {
		if !VirtualKeyReturnHostAllowed("example.com", uri) {
			t.Errorf("want allowed: %s", uri)
		}
	}
	rejected := []string{
		"https://evil.test/finish-setup",
		"https://example.com.evil.test/finish-setup",
		"https://notexample.com/finish-setup",
		"http://example.com/finish-setup",
		"https://user:pass@example.com/finish-setup",
		"javascript:alert(1)",
		"https://192.168.1.1/finish-setup",
		"",
	}
	for _, uri := range rejected {
		if VirtualKeyReturnHostAllowed("example.com", uri) {
			t.Errorf("want rejected: %s", uri)
		}
	}
}

func TestRejectVirtualKeyReturnURI(t *testing.T) {
	t.Parallel()
	for _, uri := range []string{
		"https://example.com/finish-setup",
		"https://evil.test/phish",
	} {
		err := RejectVirtualKeyReturnURI("example.com", uri)
		if !errors.Is(err, protocol.ErrVirtualKeyReturnURI) {
			t.Fatalf("RejectVirtualKeyReturnURI(%q) = %v", uri, err)
		}
	}
}
