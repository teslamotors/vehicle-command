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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/oauth2"
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

type countingTokenSource struct {
	mu     sync.Mutex
	calls  atomic.Int32
	tokens []string
	err    error
}

func (s *countingTokenSource) Token() (*oauth2.Token, error) {
	s.calls.Add(1)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return nil, s.err
	}
	if len(s.tokens) == 0 {
		return nil, fmt.Errorf("no tokens left")
	}
	tok := s.tokens[0]
	if len(s.tokens) > 1 {
		s.tokens = s.tokens[1:]
	}
	return &oauth2.Token{
		AccessToken: tok,
		TokenType:   "Bearer",
		Expiry:      time.Now().Add(-time.Minute), // force refresh on next ReuseTokenSource fetch
	}, nil
}

func validAccessToken(aud string) string {
	return makeTestJWT(&oauthPayload{
		Audiences: []string{aud},
		Subject:   "sub-1",
	})
}

func TestFromTokenSourceNil(t *testing.T) {
	if _, err := FromTokenSource(nil, ""); err == nil {
		t.Fatal("expected error for nil TokenSource")
	}
	if _, err := FromToken(nil, ""); err == nil {
		t.Fatal("expected error for nil Token")
	}
}

func TestFromTokenSourceUsesRefreshedAccessToken(t *testing.T) {
	first := validAccessToken("https://fleet-api.prd.na.vn.cloud.tesla.com")
	second := validAccessToken("https://fleet-api.prd.na.vn.cloud.tesla.com")
	src := &countingTokenSource{tokens: []string{first, second}}

	acct, err := FromTokenSource(src, "test-agent")
	if err != nil {
		t.Fatal(err)
	}
	if acct.Host != "fleet-api.prd.na.vn.cloud.tesla.com" {
		t.Fatalf("Host=%q", acct.Host)
	}
	if acct.Subject != "sub-1" {
		t.Fatalf("Subject=%q", acct.Subject)
	}
	if src.calls.Load() != 1 {
		t.Fatalf("construction should call Token once, got %d", src.calls.Load())
	}

	var authHeaders []string
	acct.client = http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			authHeaders = append(authHeaders, req.Header.Get("Authorization"))
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	// First request: ReuseTokenSource may refresh because expiry was in the past.
	if _, err := acct.Get(context.Background(), "api/1/vehicles"); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if _, err := acct.Post(context.Background(), "api/1/users/keys", []byte(`{}`)); err != nil {
		t.Fatalf("Post: %v", err)
	}
	if len(authHeaders) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(authHeaders))
	}
	for i, h := range authHeaders {
		if !strings.HasPrefix(h, "Bearer ") {
			t.Fatalf("request %d auth=%q", i, h)
		}
		if strings.Contains(h, "Bearer Bearer") {
			t.Fatalf("request %d duplicated token type: %q", i, h)
		}
	}
	// TokenSource must have been consulted again after the expired cached token.
	if src.calls.Load() < 2 {
		t.Fatalf("expected TokenSource refresh, calls=%d", src.calls.Load())
	}
}

func TestFromTokenSourcePropagatesToGetVehicle(t *testing.T) {
	access := validAccessToken("https://fleet-api.prd.na.vn.cloud.tesla.com")
	src := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: access})

	acct, err := FromTokenSource(src, "")
	if err != nil {
		t.Fatal(err)
	}

	var sawAuth string
	acct.client = http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"response":{"state":"online"}}`))),
				Header:     make(http.Header),
			}, nil
		}),
	}

	// Intercept the vehicle connection's HTTP client by building the vehicle
	// and then sending Wakeup through a custom auth path: replace connection
	// auth by exercising GetVehicle's SetAuthHeaderFunc wiring via Wakeup on a
	// connection that uses our round tripper.
	car, err := acct.GetVehicle(context.Background(), "5YJ3E1EA1KF000001", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer car.Disconnect()

	// The inet connection created by GetVehicle has its own client. Swap in a
	// recorder by sending through Account.authorization used by the connection
	// callback — verify the callback returns the static token.
	auth, err := acct.authorization()
	if err != nil {
		t.Fatal(err)
	}
	sawAuth = auth
	if sawAuth != "Bearer "+access {
		t.Fatalf("authorization=%q", sawAuth)
	}

	// Ensure GetVehicle installed a live auth func: a TokenSource error should
	// surface on Fleet API commands through the vehicle connection.
	errSrc := &countingTokenSource{err: fmt.Errorf("refresh failed")}
	acct.tokenSource = errSrc
	if err := car.Wakeup(context.Background()); err == nil || !strings.Contains(err.Error(), "refresh failed") {
		t.Fatalf("Wakeup error = %v, want refresh failure from TokenSource", err)
	}
}

func TestNewRemainsStatic(t *testing.T) {
	access := validAccessToken("https://fleet-api.prd.na.vn.cloud.tesla.com")
	acct, err := New(access, "")
	if err != nil {
		t.Fatal(err)
	}
	var got string
	acct.client = http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			got = req.Header.Get("Authorization")
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
				Header:     make(http.Header),
			}, nil
		}),
	}
	if _, err := acct.Get(context.Background(), "api/1/vehicles"); err != nil {
		t.Fatal(err)
	}
	if got != "Bearer "+access {
		t.Fatalf("Authorization=%q", got)
	}
}
