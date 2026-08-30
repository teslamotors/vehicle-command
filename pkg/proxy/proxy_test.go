package proxy

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/teslamotors/vehicle-command/pkg/account"
	"github.com/teslamotors/vehicle-command/pkg/cache"
	"github.com/teslamotors/vehicle-command/pkg/connector/inet"
	"github.com/teslamotors/vehicle-command/pkg/protocol"
	"github.com/teslamotors/vehicle-command/pkg/vehicle"
)

const (
	testVIN     = "5YJ3E1EA1KF000001"
	testSubject = "test-subject"
	testHost    = "fleet-api.prd.na.vn.cloud.tesla.com"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

type timeoutErr struct{}

func (timeoutErr) Error() string { return "timeout" }
func (timeoutErr) Timeout() bool { return true }

type mockVehicle struct {
	connectErr   error
	sessionErr   error
	executeErr   error
	executeCalls int
	connected    bool
	disconnected bool
	sessionOK    bool
	updatedCache bool
}

func (m *mockVehicle) Connect(context.Context) error {
	if m.connectErr != nil {
		return m.connectErr
	}
	m.connected = true
	return nil
}

func (m *mockVehicle) Disconnect() {
	m.disconnected = true
}

func (m *mockVehicle) StartSession(context.Context) error {
	if m.sessionErr != nil {
		return m.sessionErr
	}
	m.sessionOK = true
	return nil
}

func (m *mockVehicle) UpdateCachedSessions(*cache.SessionCache) error {
	m.updatedCache = true
	return nil
}

func (m *mockVehicle) Execute(func(*vehicle.Vehicle) error) error {
	m.executeCalls++
	return m.executeErr
}

func testPrivateKey(t *testing.T) protocol.ECDHPrivateKey {
	t.Helper()
	key, err := protocol.LoadPrivateKey("../../internal/authentication/test_data/valid_private_key.pem")
	if err != nil {
		t.Fatalf("load private key: %v", err)
	}
	return key
}

func testJWT(subject, audience string) string {
	payload := map[string]interface{}{
		"aud": []string{audience},
		"sub": subject,
	}
	body, _ := json.Marshal(payload)
	return fmt.Sprintf("x.%s.y", base64.RawStdEncoding.EncodeToString(body))
}

func authHeader() string {
	return "Bearer " + testJWT(testSubject, "https://"+testHost)
}

func newTestProxy(t *testing.T) *Proxy {
	t.Helper()
	p, err := New(context.Background(), testPrivateKey(t), 10)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	p.Timeout = time.Second
	return p
}

func jsonResponse(status int, body string, header http.Header) *http.Response {
	if header == nil {
		header = make(http.Header)
	}
	header.Set("Content-Type", "application/json")
	return &http.Response{
		StatusCode: status,
		Header:     header,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func decodeResponse(t *testing.T, body []byte) Response {
	t.Helper()
	var resp Response
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode response %q: %v", body, err)
	}
	return resp
}

func TestNew(t *testing.T) {
	p, err := New(context.Background(), testPrivateKey(t), 5)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if p.Timeout != DefaultTimeout {
		t.Errorf("Timeout = %v, want %v", p.Timeout, DefaultTimeout)
	}
	if p.commandKey == nil {
		t.Error("commandKey is nil")
	}
	if p.sessions == nil {
		t.Error("sessions is nil")
	}
	if p.client == nil {
		t.Error("client is nil")
	}
	if p.fetchVehicle == nil {
		t.Error("fetchVehicle is nil")
	}
}

func TestDomainForSubject(t *testing.T) {
	p := newTestProxy(t)
	if got := p.fetchDomainForSubject(testSubject); got != "" {
		t.Errorf("unexpected domain %q", got)
	}
	p.updateDomainForSubject(testSubject, "alt.example.com")
	if got := p.fetchDomainForSubject(testSubject); got != "alt.example.com" {
		t.Errorf("domain = %q, want alt.example.com", got)
	}
}

func TestUnsupportedVIN(t *testing.T) {
	p := newTestProxy(t)
	if p.isNotSupported(testVIN) {
		t.Fatal("VIN unexpectedly marked unsupported")
	}
	p.markUnsupportedVIN(testVIN)
	if !p.isNotSupported(testVIN) {
		t.Fatal("VIN should be marked unsupported")
	}
}

func TestLockVIN(t *testing.T) {
	p := newTestProxy(t)
	ctx := context.Background()
	if err := p.lockVIN(ctx, testVIN); err != nil {
		t.Fatalf("lockVIN: %v", err)
	}

	locked := make(chan struct{})
	errCh := make(chan error, 1)
	go func() {
		close(locked)
		shortCtx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
		defer cancel()
		errCh <- p.lockVIN(shortCtx, testVIN)
	}()
	<-locked
	if err := <-errCh; !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("second lockVIN error = %v, want deadline exceeded", err)
	}

	p.unlockVIN(testVIN)
	if err := p.lockVIN(ctx, testVIN); err != nil {
		t.Fatalf("lockVIN after unlock: %v", err)
	}
	p.unlockVIN(testVIN)
}

func TestUnlockVINPanicsWithoutLock(t *testing.T) {
	p := newTestProxy(t)
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	p.unlockVIN(testVIN)
}

func TestWriteJSONError(t *testing.T) {
	t.Run("nil error uses status text", func(t *testing.T) {
		rec := httptest.NewRecorder()
		writeJSONError(rec, http.StatusMethodNotAllowed, nil)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("code = %d", rec.Code)
		}
		resp := decodeResponse(t, rec.Body.Bytes())
		if resp.Error != http.StatusText(http.StatusMethodNotAllowed) {
			t.Fatalf("error = %q", resp.Error)
		}
	})

	t.Run("generic error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		writeJSONError(rec, http.StatusBadRequest, errors.New("boom"))
		resp := decodeResponse(t, rec.Body.Bytes())
		if resp.Error != "boom" {
			t.Fatalf("error = %q", resp.Error)
		}
	})

	t.Run("nominal error embeds car response", func(t *testing.T) {
		rec := httptest.NewRecorder()
		writeJSONError(rec, http.StatusOK, &protocol.NominalError{Details: errors.New("already locked")})
		if rec.Code != http.StatusOK {
			t.Fatalf("code = %d", rec.Code)
		}
		resp := decodeResponse(t, rec.Body.Bytes())
		raw, err := json.Marshal(resp.Response)
		if err != nil {
			t.Fatal(err)
		}
		var car carResponse
		if err := json.Unmarshal(raw, &car); err != nil {
			t.Fatal(err)
		}
		if car.Reason != "already locked" {
			t.Fatalf("reason = %q", car.Reason)
		}
	})

	t.Run("http error passes through body and status", func(t *testing.T) {
		rec := httptest.NewRecorder()
		httpErr := &inet.HTTPError{Code: http.StatusTeapot, Message: `{"error":"teapot"}`}
		writeJSONError(rec, http.StatusBadRequest, httpErr)
		if rec.Code != http.StatusTeapot {
			t.Fatalf("code = %d", rec.Code)
		}
		if strings.TrimSpace(rec.Body.String()) != `{"error":"teapot"}` {
			t.Fatalf("body = %q", rec.Body.String())
		}
	})
}

func TestGetAccount(t *testing.T) {
	t.Run("missing bearer", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/1/vehicles", nil)
		if _, err := getAccount(req); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("malformed token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/1/vehicles", nil)
		req.Header.Set("Authorization", "Bearer not-a-jwt")
		if _, err := getAccount(req); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("valid token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/1/vehicles", nil)
		req.Header.Set("Authorization", authHeader())
		acct, err := getAccount(req)
		if err != nil {
			t.Fatalf("getAccount: %v", err)
		}
		if acct.Host != testHost {
			t.Fatalf("host = %q", acct.Host)
		}
		if acct.Subject != testSubject {
			t.Fatalf("subject = %q", acct.Subject)
		}
	})
}

func TestHandleHealthCheck(t *testing.T) {
	p := newTestProxy(t)

	t.Run("get", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		p.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || rec.Body.String() != "OK" {
			t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("post not allowed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/health", nil)
		p.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}

func TestServeHTTPAuthRequired(t *testing.T) {
	p := newTestProxy(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/1/vehicles", nil)
	p.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestServeHTTPInvalidVIN(t *testing.T) {
	p := newTestProxy(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/1/vehicles/SHORTVIN/command/door_lock", nil)
	req.Header.Set("Authorization", authHeader())
	p.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	resp := decodeResponse(t, rec.Body.Bytes())
	if !strings.Contains(resp.Error, "17-character VIN") {
		t.Fatalf("error = %q", resp.Error)
	}
}

func TestForwardRequest(t *testing.T) {
	p := newTestProxy(t)
	var gotReq *http.Request
	var gotBody []byte
	p.client = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		gotReq = req.Clone(req.Context())
		if req.Body != nil {
			var err error
			gotBody, err = io.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
		}
		header := make(http.Header)
		header.Set("X-Upstream", "yes")
		header.Set("Keep-Alive", "timeout=5")
		return jsonResponse(http.StatusOK, `{"ok":true}`, header), nil
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/1/vehicles", strings.NewReader(`{"hello":"world"}`))
	req.Header.Set("Authorization", authHeader())
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	req.Header.Set("Keep-Alive", "timeout=5")
	req.Header.Set("Proxy-Connection", "keep-alive")
	req.RemoteAddr = "9.9.9.9:1234"
	p.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != `{"ok":true}` {
		t.Fatalf("body=%q", rec.Body.String())
	}
	if rec.Header().Get("X-Upstream") != "yes" {
		t.Fatal("missing upstream header")
	}
	if rec.Header().Get("Keep-Alive") != "" {
		t.Fatal("hop-by-hop header should be stripped from response")
	}
	if gotReq == nil {
		t.Fatal("upstream request not captured")
	}
	if gotReq.URL.Scheme != "https" || gotReq.URL.Host != testHost {
		t.Fatalf("upstream URL = %s", gotReq.URL)
	}
	if gotReq.Header.Get("Keep-Alive") != "" || gotReq.Header.Get("Proxy-Connection") != "" {
		t.Fatal("hop-by-hop headers should be stripped from request")
	}
	if gotReq.Header.Get("X-Forwarded-For") != "1.2.3.4, 9.9.9.9" {
		t.Fatalf("XFF = %q", gotReq.Header.Get("X-Forwarded-For"))
	}
	if string(gotBody) != `{"hello":"world"}` {
		t.Fatalf("body = %q", gotBody)
	}
}

func TestForwardRequestAddsXFFWhenMissing(t *testing.T) {
	p := newTestProxy(t)
	var xff string
	p.client = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		xff = req.Header.Get("X-Forwarded-For")
		return jsonResponse(http.StatusOK, `{}`, nil), nil
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/1/vehicles", nil)
	req.Header.Set("Authorization", authHeader())
	req.RemoteAddr = "8.8.8.8:443"
	p.ServeHTTP(rec, req)
	if xff != "8.8.8.8" {
		t.Fatalf("XFF = %q", xff)
	}
}

func TestForwardRequestUsesCachedDomain(t *testing.T) {
	p := newTestProxy(t)
	p.updateDomainForSubject(testSubject, "cached.example.tesla.com")
	var host string
	p.client = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		host = req.URL.Host
		return jsonResponse(http.StatusOK, `{}`, nil), nil
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/1/vehicles", nil)
	req.Header.Set("Authorization", authHeader())
	p.ServeHTTP(rec, req)
	if host != "cached.example.tesla.com" {
		t.Fatalf("host = %q", host)
	}
}

func TestForwardRequestUpstreamErrors(t *testing.T) {
	t.Run("timeout", func(t *testing.T) {
		p := newTestProxy(t)
		p.client = roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, &url.Error{Op: "Post", URL: "https://example", Err: timeoutErr{}}
		})
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/1/vehicles", nil)
		req.Header.Set("Authorization", authHeader())
		p.ServeHTTP(rec, req)
		if rec.Code != http.StatusGatewayTimeout {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("generic transport error", func(t *testing.T) {
		p := newTestProxy(t)
		p.client = roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("connection reset")
		})
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/1/vehicles", nil)
		req.Header.Set("Authorization", authHeader())
		p.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadGateway {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("response body read error", func(t *testing.T) {
		p := newTestProxy(t)
		p.client = roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(&errReader{}),
			}, nil
		})
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/1/vehicles", nil)
		req.Header.Set("Authorization", authHeader())
		p.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadGateway {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("response too large", func(t *testing.T) {
		p := newTestProxy(t)
		p.client = roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(bytes.NewReader(bytes.Repeat([]byte("a"), MaxResponseLength+1))),
			}, nil
		})
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/1/vehicles", nil)
		req.Header.Set("Authorization", authHeader())
		p.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadGateway {
			t.Fatalf("status=%d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "response exceeds maximum length") {
			t.Fatalf("body=%s", rec.Body.String())
		}
	})
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

func TestForwardRequestAltSvcRetry(t *testing.T) {
	p := newTestProxy(t)
	p.Timeout = 5 * time.Second
	var hosts []string
	var bodies []string
	attempts := 0
	p.client = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		attempts++
		hosts = append(hosts, req.URL.Host)
		body, _ := io.ReadAll(req.Body)
		bodies = append(bodies, string(body))
		if attempts == 1 {
			header := make(http.Header)
			header.Add("Alt-Svc", "h3=\":443\"")
			header.Add("Alt-Svc", h2Prefix+"alt.example.tesla.com")
			return jsonResponse(http.StatusMisdirectedRequest, `{"error":"misdirected"}`, header), nil
		}
		return jsonResponse(http.StatusOK, `{"ok":true}`, nil), nil
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/1/vehicles", strings.NewReader(`{"n":1}`))
	req.Header.Set("Authorization", authHeader())
	p.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(hosts) != 2 || hosts[0] != testHost || hosts[1] != "alt.example.tesla.com" {
		t.Fatalf("hosts = %#v", hosts)
	}
	if bodies[0] != `{"n":1}` || bodies[1] != `{"n":1}` {
		t.Fatalf("bodies = %#v", bodies)
	}
	if got := p.fetchDomainForSubject(testSubject); got != "alt.example.tesla.com" {
		t.Fatalf("cached domain = %q", got)
	}
}

func TestForwardRequestAltSvcMissingH2(t *testing.T) {
	p := newTestProxy(t)
	p.client = roundTripFunc(func(*http.Request) (*http.Response, error) {
		header := make(http.Header)
		header.Set("Alt-Svc", "h3=\":443\"")
		return jsonResponse(http.StatusMisdirectedRequest, `{}`, header), nil
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/1/vehicles", nil)
	req.Header.Set("Authorization", authHeader())
	p.ServeHTTP(rec, req)
	if rec.Code != http.StatusMisdirectedRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestForwardRequestMaxRetryExhausted(t *testing.T) {
	p := newTestProxy(t)
	p.Timeout = 5 * time.Second
	p.client = roundTripFunc(func(*http.Request) (*http.Response, error) {
		header := make(http.Header)
		header.Set("Alt-Svc", h2Prefix+"alt.example.tesla.com")
		return jsonResponse(http.StatusMisdirectedRequest, `{}`, header), nil
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/1/vehicles", nil)
	req.Header.Set("Authorization", authHeader())
	start := time.Now()
	p.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "max retry exhausted") {
		t.Fatalf("body=%s", rec.Body.String())
	}
	// One sleep between the first and second attempt.
	if time.Since(start) < time.Second {
		t.Fatalf("expected retry delay, elapsed %s", time.Since(start))
	}
}

func TestForwardRequestRetryContextTimeout(t *testing.T) {
	p := newTestProxy(t)
	p.Timeout = 100 * time.Millisecond
	p.client = roundTripFunc(func(*http.Request) (*http.Response, error) {
		header := make(http.Header)
		header.Set("Alt-Svc", h2Prefix+"alt.example.tesla.com")
		return jsonResponse(http.StatusMisdirectedRequest, `{}`, header), nil
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/1/vehicles", nil)
	req.Header.Set("Authorization", authHeader())
	p.ServeHTTP(rec, req)
	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestServeHTTPUnsupportedVINForwards(t *testing.T) {
	p := newTestProxy(t)
	p.markUnsupportedVIN(testVIN)
	var path string
	p.client = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		path = req.URL.Path
		return jsonResponse(http.StatusOK, `{"forwarded":true}`, nil), nil
	})
	// Pretend upstream also updated the account host during forwarding.
	p.fetchVehicle = func(context.Context, *account.Account, string) (vehicleSession, error) {
		t.Fatal("fetchVehicle should not be called for unsupported VIN")
		return nil, nil
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/1/vehicles/"+testVIN+"/command/door_lock", strings.NewReader(`{}`))
	req.Header.Set("Authorization", authHeader())
	p.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if path != "/api/1/vehicles/"+testVIN+"/command/door_lock" {
		t.Fatalf("path=%q", path)
	}
}

func TestServeHTTPUnsupportedVINUpdatesDomain(t *testing.T) {
	p := newTestProxy(t)
	p.markUnsupportedVIN(testVIN)
	p.client = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		header := make(http.Header)
		header.Set("Alt-Svc", h2Prefix+"from-unsupported.tesla.com")
		if req.URL.Host == testHost {
			return jsonResponse(http.StatusMisdirectedRequest, `{}`, header), nil
		}
		return jsonResponse(http.StatusOK, `{}`, nil), nil
	})
	p.Timeout = 5 * time.Second

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/1/vehicles/"+testVIN+"/command/door_lock", nil)
	req.Header.Set("Authorization", authHeader())
	p.ServeHTTP(rec, req)
	if got := p.fetchDomainForSubject(testSubject); got != "from-unsupported.tesla.com" {
		t.Fatalf("cached domain = %q", got)
	}
}

func TestVehicleCommandSuccess(t *testing.T) {
	p := newTestProxy(t)
	mock := &mockVehicle{}
	p.fetchVehicle = func(context.Context, *account.Account, string) (vehicleSession, error) {
		return mock, nil
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/1/vehicles/"+testVIN+"/command/door_lock", nil)
	req.Header.Set("Authorization", authHeader())
	p.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"result":true`) {
		t.Fatalf("body=%s", rec.Body.String())
	}
	if !mock.connected || !mock.sessionOK || mock.executeCalls != 1 || !mock.disconnected || !mock.updatedCache {
		t.Fatalf("mock state: %+v", mock)
	}
}

func TestVehicleCommandMethodNotAllowed(t *testing.T) {
	p := newTestProxy(t)
	p.fetchVehicle = func(context.Context, *account.Account, string) (vehicleSession, error) {
		t.Fatal("should not fetch vehicle")
		return nil, nil
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/1/vehicles/"+testVIN+"/command/door_lock", nil)
	req.Header.Set("Authorization", authHeader())
	p.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestVehicleCommandBadJSON(t *testing.T) {
	p := newTestProxy(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/1/vehicles/"+testVIN+"/command/adjust_volume", strings.NewReader(`{`))
	req.Header.Set("Authorization", authHeader())
	p.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestVehicleCommandInvalidCommand(t *testing.T) {
	p := newTestProxy(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/1/vehicles/"+testVIN+"/command/not_a_real_command", nil)
	req.Header.Set("Authorization", authHeader())
	p.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "invalid_command") {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

func TestVehicleCommandFetchErrors(t *testing.T) {
	t.Run("fetch error", func(t *testing.T) {
		p := newTestProxy(t)
		p.fetchVehicle = func(context.Context, *account.Account, string) (vehicleSession, error) {
			return nil, errors.New("no vehicle")
		}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/1/vehicles/"+testVIN+"/command/door_lock", nil)
		req.Header.Set("Authorization", authHeader())
		p.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("nil vehicle", func(t *testing.T) {
		p := newTestProxy(t)
		p.fetchVehicle = func(context.Context, *account.Account, string) (vehicleSession, error) {
			return nil, nil
		}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/1/vehicles/"+testVIN+"/command/door_lock", nil)
		req.Header.Set("Authorization", authHeader())
		p.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}

func TestVehicleCommandConnectAndSessionErrors(t *testing.T) {
	t.Run("connect error", func(t *testing.T) {
		p := newTestProxy(t)
		p.fetchVehicle = func(context.Context, *account.Account, string) (vehicleSession, error) {
			return &mockVehicle{connectErr: errors.New("connect failed")}, nil
		}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/1/vehicles/"+testVIN+"/command/door_lock", nil)
		req.Header.Set("Authorization", authHeader())
		p.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("session error", func(t *testing.T) {
		p := newTestProxy(t)
		p.fetchVehicle = func(context.Context, *account.Account, string) (vehicleSession, error) {
			return &mockVehicle{sessionErr: errors.New("handshake failed")}, nil
		}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/1/vehicles/"+testVIN+"/command/door_lock", nil)
		req.Header.Set("Authorization", authHeader())
		p.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})
}

func TestVehicleCommandProtocolNotSupportedFallsBack(t *testing.T) {
	p := newTestProxy(t)
	mock := &mockVehicle{sessionErr: protocol.ErrProtocolNotSupported}
	p.fetchVehicle = func(context.Context, *account.Account, string) (vehicleSession, error) {
		return mock, nil
	}
	var forwardedBody string
	var forwardedPath string
	p.client = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		forwardedPath = req.URL.Path
		body, _ := io.ReadAll(req.Body)
		forwardedBody = string(body)
		return jsonResponse(http.StatusOK, `{"rest":true}`, nil), nil
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/1/vehicles/"+testVIN+"/command/door_lock", strings.NewReader(`{"x":1}`))
	req.Header.Set("Authorization", authHeader())
	p.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"rest":true`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !p.isNotSupported(testVIN) {
		t.Fatal("VIN should be marked unsupported")
	}
	if forwardedPath != "/api/1/vehicles/"+testVIN+"/command/door_lock" {
		t.Fatalf("path=%q", forwardedPath)
	}
	if forwardedBody != `{"x":1}` {
		t.Fatalf("forwarded body=%q", forwardedBody)
	}
}

func TestVehicleCommandUseRESTAPIFallsBack(t *testing.T) {
	p := newTestProxy(t)
	var forwardedBody string
	p.client = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(req.Body)
		forwardedBody = string(body)
		return jsonResponse(http.StatusOK, `{"rest":true}`, nil), nil
	})
	p.fetchVehicle = func(context.Context, *account.Account, string) (vehicleSession, error) {
		t.Fatal("REST-only command should not open a vehicle session")
		return nil, nil
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/1/vehicles/"+testVIN+"/command/navigation_request", strings.NewReader(`{"locale":"en"}`))
	req.Header.Set("Authorization", authHeader())
	p.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if forwardedBody != `{"locale":"en"}` {
		t.Fatalf("forwarded body=%q", forwardedBody)
	}
}

func TestVehicleCommandExecuteErrors(t *testing.T) {
	t.Run("nominal error", func(t *testing.T) {
		p := newTestProxy(t)
		p.fetchVehicle = func(context.Context, *account.Account, string) (vehicleSession, error) {
			return &mockVehicle{executeErr: &protocol.NominalError{Details: errors.New("already locked")}}, nil
		}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/1/vehicles/"+testVIN+"/command/door_lock", nil)
		req.Header.Set("Authorization", authHeader())
		p.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
		resp := decodeResponse(t, rec.Body.Bytes())
		raw, _ := json.Marshal(resp.Response)
		if !strings.Contains(string(raw), "already locked") {
			t.Fatalf("response=%s", raw)
		}
	})

	t.Run("fatal error", func(t *testing.T) {
		p := newTestProxy(t)
		p.fetchVehicle = func(context.Context, *account.Account, string) (vehicleSession, error) {
			return &mockVehicle{executeErr: errors.New("bus error")}, nil
		}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/1/vehicles/"+testVIN+"/command/door_lock", nil)
		req.Header.Set("Authorization", authHeader())
		p.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("execute returns ErrCommandUseRESTAPI", func(t *testing.T) {
		p := newTestProxy(t)
		p.fetchVehicle = func(context.Context, *account.Account, string) (vehicleSession, error) {
			return &mockVehicle{executeErr: ErrCommandUseRESTAPI}, nil
		}
		var forwarded bool
		p.client = roundTripFunc(func(*http.Request) (*http.Response, error) {
			forwarded = true
			return jsonResponse(http.StatusOK, `{"rest":true}`, nil), nil
		})
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/1/vehicles/"+testVIN+"/command/door_lock", nil)
		req.Header.Set("Authorization", authHeader())
		p.ServeHTTP(rec, req)
		if !forwarded || rec.Code != http.StatusOK {
			t.Fatalf("forwarded=%v status=%d body=%s", forwarded, rec.Code, rec.Body.String())
		}
	})
}

func TestVehicleCommandLockTimeout(t *testing.T) {
	p := newTestProxy(t)
	p.Timeout = 40 * time.Millisecond
	if err := p.lockVIN(context.Background(), testVIN); err != nil {
		t.Fatal(err)
	}
	defer p.unlockVIN(testVIN)

	called := false
	p.fetchVehicle = func(context.Context, *account.Account, string) (vehicleSession, error) {
		called = true
		return &mockVehicle{}, nil
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/1/vehicles/"+testVIN+"/command/door_lock", nil)
	req.Header.Set("Authorization", authHeader())
	p.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if called {
		t.Fatal("should not fetch vehicle when VIN lock times out")
	}
}

func TestVehicleCommandSerializesPerVIN(t *testing.T) {
	p := newTestProxy(t)
	var mu sync.Mutex
	inFlight := 0
	maxInFlight := 0
	release := make(chan struct{})

	p.fetchVehicle = func(context.Context, *account.Account, string) (vehicleSession, error) {
		return &blockingVehicle{
			onExecute: func() {
				mu.Lock()
				inFlight++
				if inFlight > maxInFlight {
					maxInFlight = inFlight
				}
				mu.Unlock()
				<-release
				mu.Lock()
				inFlight--
				mu.Unlock()
			},
		}, nil
	}

	var wg sync.WaitGroup
	results := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/1/vehicles/"+testVIN+"/command/door_lock", nil)
			req.Header.Set("Authorization", authHeader())
			p.ServeHTTP(rec, req)
			results <- rec.Code
		}()
	}

	deadline := time.After(time.Second)
	for {
		mu.Lock()
		n := inFlight
		mu.Unlock()
		if n == 1 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for first command to hold VIN lock")
		case <-time.After(5 * time.Millisecond):
		}
	}
	close(release)
	wg.Wait()
	close(results)
	for code := range results {
		if code != http.StatusOK {
			t.Fatalf("status=%d", code)
		}
	}
	if maxInFlight != 1 {
		t.Fatalf("maxInFlight=%d, want 1", maxInFlight)
	}
}

type blockingVehicle struct {
	onExecute func()
}

func (b *blockingVehicle) Connect(context.Context) error      { return nil }
func (b *blockingVehicle) Disconnect()                        {}
func (b *blockingVehicle) StartSession(context.Context) error { return nil }
func (b *blockingVehicle) UpdateCachedSessions(*cache.SessionCache) error {
	return nil
}
func (b *blockingVehicle) Execute(func(*vehicle.Vehicle) error) error {
	b.onExecute()
	return nil
}

func TestHandleFleetTelemetryConfig(t *testing.T) {
	p := newTestProxy(t)
	var upstreamPath string
	var upstreamBody map[string]interface{}
	p.client = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		upstreamPath = req.URL.Path
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(body, &upstreamBody); err != nil {
			return nil, err
		}
		return jsonResponse(http.StatusOK, `{"config":"ok"}`, nil), nil
	})

	payload := map[string]interface{}{
		"vins":   []string{testVIN},
		"config": jwt.MapClaims{"aud": "will-be-overwritten", "iss": "also-overwritten", "foo": "bar"},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/1/vehicles/fleet_telemetry_config", bytes.NewReader(body))
	req.Header.Set("Authorization", authHeader())
	p.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if upstreamPath != "/api/1/vehicles/fleet_telemetry_config_jws" {
		t.Fatalf("path=%q", upstreamPath)
	}
	token, _ := upstreamBody["token"].(string)
	if token == "" {
		t.Fatalf("missing token in %#v", upstreamBody)
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("token is not a JWT: %q", token)
	}
	vins, _ := upstreamBody["vins"].([]interface{})
	if len(vins) != 1 || vins[0] != testVIN {
		t.Fatalf("vins=%#v", vins)
	}
}

func TestHandleFleetTelemetryConfigErrors(t *testing.T) {
	p := newTestProxy(t)

	t.Run("invalid json", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/1/vehicles/fleet_telemetry_config", strings.NewReader(`{`))
		req.Header.Set("Authorization", authHeader())
		p.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d", rec.Code)
		}
	})

	t.Run("body read error", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/1/vehicles/fleet_telemetry_config", errReadCloser{})
		req.Header.Set("Authorization", authHeader())
		p.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("signing failure", func(t *testing.T) {
		p := newTestProxy(t)
		p.commandKey = failingKey{}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/1/vehicles/fleet_telemetry_config", strings.NewReader(`{"vins":[],"config":{}}`))
		req.Header.Set("Authorization", authHeader())
		p.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "error signing configuration") {
			t.Fatalf("body=%s", rec.Body.String())
		}
	})
}

type failingKey struct{}

func (failingKey) PublicBytes() []byte { return []byte{1, 2, 3} }
func (failingKey) Exchange([]byte) (protocol.Session, error) {
	return nil, errors.New("unused")
}
func (failingKey) SchnorrSignature([]byte) ([]byte, error) {
	return nil, errors.New("sign failed")
}

type errReadCloser struct{}

func (errReadCloser) Read([]byte) (int, error) { return 0, errors.New("boom") }
func (errReadCloser) Close() error             { return nil }

func TestExtractCommandActionReadsAndRestoresBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"volume":3}`))
	fn, err := extractCommandAction(context.Background(), req, "adjust_volume")
	if err != nil {
		t.Fatalf("extractCommandAction: %v", err)
	}
	if fn == nil {
		t.Fatal("expected function")
	}
	restored, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(restored) != `{"volume":3}` {
		t.Fatalf("restored body=%q", restored)
	}
}

func TestExtractCommandActionBodyReadError(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", errReadCloser{})
	_, err := extractCommandAction(context.Background(), req, "door_lock")
	var httpErr *inet.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Code != http.StatusBadRequest {
		t.Fatalf("err=%v", err)
	}
}

func TestForwardRequestBadRemoteAddr(t *testing.T) {
	p := newTestProxy(t)
	p.client = roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("should not forward with invalid RemoteAddr")
		return nil, nil
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/1/vehicles", nil)
	req.Header.Set("Authorization", authHeader())
	req.RemoteAddr = "not-a-host-port"
	p.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestForwardRequestBodyReadError(t *testing.T) {
	p := newTestProxy(t)
	p.client = roundTripFunc(func(*http.Request) (*http.Response, error) {
		t.Fatal("should not forward when body cannot be read")
		return nil, nil
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/1/vehicles", errReadCloser{})
	req.Header.Set("Authorization", authHeader())
	req.RemoteAddr = "1.1.1.1:80"
	p.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestDefaultFetchVehicleAndLiveVehicle(t *testing.T) {
	p := newTestProxy(t)
	acct, err := account.New(testJWT(testSubject, "https://"+testHost), proxyProtocolVersion)
	if err != nil {
		t.Fatal(err)
	}
	session, err := p.defaultFetchVehicle(context.Background(), acct, testVIN)
	if err != nil {
		t.Fatalf("defaultFetchVehicle: %v", err)
	}
	live, ok := session.(*liveVehicle)
	if !ok || live.Vehicle == nil {
		t.Fatalf("got %#v", session)
	}
	if err := session.Execute(func(v *vehicle.Vehicle) error {
		if v == nil {
			return errors.New("nil vehicle")
		}
		return nil
	}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
}
