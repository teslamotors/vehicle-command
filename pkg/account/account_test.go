package account

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// b64Encode encodes a string to base64 without padding.
func b64Encode(payload string) string {
	return base64.RawStdEncoding.EncodeToString([]byte(payload))
}

// TestNewAccount tests the creation of a new account with various JWT scenarios.
func TestNewAccount(t *testing.T) {
	validDomain := "fleet-api.example.tesla.com"

	tests := []struct {
		jwt         string
		shouldError bool
		description string
	}{
		{"", true, "empty JWT"},
		{b64Encode(validDomain), true, "one-field JWT"},
		{"x." + b64Encode(validDomain), true, "two-field JWT"},
		{"x." + b64Encode(validDomain) + "y.z", true, "four-field JWT"},
		{"x." + validDomain + ".y", true, "non-base64 encoded JWT"},
		{"x." + b64Encode("{\"aud\": \"example.com\"}") + ".y", true, "untrusted domain"},
		{"x." + b64Encode(fmt.Sprintf("{\"aud\": \"%s\"}", validDomain)) + ".y", true, "aud field not a list"},
		{"x." + b64Encode(fmt.Sprintf("{\"aud\": [\"%s\"]}", validDomain)) + ".y", false, "valid JWT"},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			acct, err := New(test.jwt, "")
			if (err != nil) != test.shouldError {
				t.Errorf("Unexpected result: err = %v, shouldError = %v", err, test.shouldError)
			}
			if !test.shouldError && (acct == nil || acct.Host != validDomain) {
				t.Errorf("acct = %+v, expected Host = %s", acct, validDomain)
			}
		})
	}
}

// TestDomainDefault tests the default domain extraction.
func TestDomainDefault(t *testing.T) {
	payload := &oauthPayload{
		Audiences: []string{"https://auth.tesla.com/nts"},
	}

	acct, err := New(makeTestJWT(payload), "")
	if err != nil {
		t.Fatalf("Returned error on valid JWT: %s", err)
	}
	if acct == nil || acct.Host != defaultDomain {
		t.Errorf("acct = %+v, expected Host = %s", acct, defaultDomain)
	}
}

// TestDomainExtraction tests the extraction of the correct domain based on OUCode.
func TestDomainExtraction(t *testing.T) {
	payload := &oauthPayload{
		Audiences: []string{
			"https://auth.tesla.com/nts",
			"https://fleet-api.prd.na.vn.cloud.tesla.com",
			"https://fleet-api.prd.eu.vn.cloud.tesla.com",
		},
		OUCode:  "EU",
		Subject: "SUBJECT",
	}

	acct, err := New(makeTestJWT(payload), "")
	if err != nil {
		t.Fatalf("Returned error on valid JWT: %s", err)
	}
	expectedHost := "fleet-api.prd.eu.vn.cloud.tesla.com"
	if acct == nil || acct.Host != expectedHost || acct.Subject != "SUBJECT" {
		t.Errorf("acct = %+v, expected Host = %s", acct, expectedHost)
	}
}

// makeTestJWT creates a JWT string with the given payload.
func makeTestJWT(payload *oauthPayload) string {
	jwtBody, _ := json.Marshal(payload)
	return fmt.Sprintf("x.%s.y", b64Encode(string(jwtBody)))
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonHTTPResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

// TestSetHTTPClientGetUsesCustomTransport reproduces #23: applications need to
// observe/log Fleet API traffic via a custom RoundTripper without mutating
// http.DefaultClient. Before SetHTTPClient existed, Account always used an
// unexported default client and there was no public API to replace it.
func TestSetHTTPClientGetUsesCustomTransport(t *testing.T) {
	acct, err := New(makeTestJWT(&oauthPayload{
		Audiences: []string{"https://fleet-api.prd.na.vn.cloud.tesla.com"},
	}), "test-agent")
	if err != nil {
		t.Fatal(err)
	}

	var sawURL string
	acct.SetHTTPClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			sawURL = req.URL.String()
			if req.Header.Get("Authorization") == "" {
				t.Error("expected Authorization header")
			}
			return jsonHTTPResponse(http.StatusOK, `{"response":[]}`), nil
		}),
	})

	body, err := acct.Get(context.Background(), "api/1/vehicles")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !bytes.Contains(body, []byte(`"response"`)) {
		t.Fatalf("unexpected body %s", body)
	}
	want := "https://fleet-api.prd.na.vn.cloud.tesla.com/api/1/vehicles"
	if sawURL != want {
		t.Fatalf("custom transport URL = %q, want %q", sawURL, want)
	}
}

func TestSetHTTPClientPostUsesCustomTransport(t *testing.T) {
	acct, err := New(makeTestJWT(&oauthPayload{
		Audiences: []string{"https://fleet-api.prd.na.vn.cloud.tesla.com"},
	}), "")
	if err != nil {
		t.Fatal(err)
	}

	var method, path string
	acct.SetHTTPClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			method = req.Method
			path = req.URL.Path
			return jsonHTTPResponse(http.StatusOK, `{"ok":true}`), nil
		}),
	})

	if _, err := acct.Post(context.Background(), "api/1/users/keys", []byte(`{}`)); err != nil {
		t.Fatalf("Post: %v", err)
	}
	if method != http.MethodPost {
		t.Fatalf("method = %q, want POST", method)
	}
	if path != "/api/1/users/keys" {
		t.Fatalf("path = %q, want /api/1/users/keys", path)
	}
}

func TestSetHTTPClientNilRestoresDefault(t *testing.T) {
	acct, err := New(makeTestJWT(&oauthPayload{
		Audiences: []string{"https://fleet-api.prd.na.vn.cloud.tesla.com"},
	}), "")
	if err != nil {
		t.Fatal(err)
	}

	blocked := &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			t.Fatal("blocked transport should not be used after SetHTTPClient(nil)")
			return nil, nil
		}),
	}
	acct.SetHTTPClient(blocked)
	acct.SetHTTPClient(nil)

	// After nil reset, requests use a fresh default client. Point Host at a
	// non-routable address so we exercise the default client path without a
	// custom RoundTripper; connection refused / timeout proves the blocked
	// transport was not invoked.
	acct.Host = "127.0.0.1:1"
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, err = acct.Get(ctx, "api/1/vehicles")
	if err == nil {
		t.Fatal("expected error from default client against closed port")
	}
}

func TestSetHTTPClientPropagatesToGetVehicle(t *testing.T) {
	acct, err := New(makeTestJWT(&oauthPayload{
		Audiences: []string{"https://fleet-api.prd.na.vn.cloud.tesla.com"},
	}), "")
	if err != nil {
		t.Fatal(err)
	}

	var paths []string
	acct.SetHTTPClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			paths = append(paths, req.URL.Path)
			return jsonHTTPResponse(http.StatusOK, `{"response":{"state":"online"}}`), nil
		}),
	})

	car, err := acct.GetVehicle(context.Background(), "5YJ3E1EA1KF000001", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer car.Disconnect()

	if err := car.Wakeup(context.Background()); err != nil {
		t.Fatalf("Wakeup: %v", err)
	}
	if len(paths) == 0 || !strings.Contains(paths[0], "/wake_up") {
		t.Fatalf("GetVehicle did not use Account HTTP client; paths=%v", paths)
	}
}
