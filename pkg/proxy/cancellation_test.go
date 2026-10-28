package proxy

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/teslamotors/vehicle-command/pkg/account"
	"github.com/teslamotors/vehicle-command/pkg/cache"
	"github.com/teslamotors/vehicle-command/pkg/protocol"
	"github.com/teslamotors/vehicle-command/pkg/vehicle"
)

type cancellationVehicle struct {
	connect       func(context.Context) error
	session       func(context.Context) error
	execute       func()
	connects      int
	sessions      int
	executions    int
	disconnects   int
	cacheUpdates  int
	cleanupEvents []string
}

func (v *cancellationVehicle) Connect(ctx context.Context) error {
	v.connects++
	if v.connect != nil {
		return v.connect(ctx)
	}
	return nil
}

func (v *cancellationVehicle) StartSession(ctx context.Context) error {
	v.sessions++
	if v.session != nil {
		return v.session(ctx)
	}
	return nil
}

func (v *cancellationVehicle) Execute(func(*vehicle.Vehicle) error) error {
	v.executions++
	if v.execute != nil {
		v.execute()
	}
	return nil
}

func (v *cancellationVehicle) Disconnect() {
	v.disconnects++
	v.cleanupEvents = append(v.cleanupEvents, "disconnect")
}

func (v *cancellationVehicle) UpdateCachedSessions(*cache.SessionCache) error {
	v.cacheUpdates++
	v.cleanupEvents = append(v.cleanupEvents, "cache")
	return nil
}

func serveCancellationCommand(ctx context.Context, p *Proxy, vin string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/1/vehicles/"+vin+"/command/door_lock", nil).WithContext(ctx)
	req.Header.Set("Authorization", authHeader())
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)
	return rec
}

func waitCancellationSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for request lifecycle")
	}
}

func TestVehicleCommandCancellationBeforeLookup(t *testing.T) {
	p := newTestProxy(t)
	lookups := 0
	p.fetchVehicle = func(context.Context, *account.Account, string) (vehicleSession, error) {
		lookups++
		return &cancellationVehicle{}, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rec := serveCancellationCommand(ctx, p, testVIN)
	if rec.Code != http.StatusServiceUnavailable || lookups != 0 {
		t.Fatalf("status=%d lookups=%d, want 503 and no lookup", rec.Code, lookups)
	}
	if _, locked := p.vinLock.Load(testVIN); locked {
		t.Fatal("canceled request retained the VIN lock")
	}
}

func TestVehicleCommandCancellationDuringSetup(t *testing.T) {
	for _, stage := range []string{"lookup", "lookup_success", "connect", "connect_success", "session", "session_success"} {
		t.Run(stage, func(t *testing.T) {
			p := newTestProxy(t)
			p.Timeout = 30 * time.Second
			v := &cancellationVehicle{}
			entered := make(chan struct{})
			release := make(chan struct{})
			observed := make(chan error, 1)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			block := func(ctx context.Context) error {
				close(entered)
				<-release
				observed <- ctx.Err()
				if stage == "lookup_success" || stage == "connect_success" || stage == "session_success" {
					return nil
				}
				return ctx.Err()
			}
			p.fetchVehicle = func(ctx context.Context, _ *account.Account, _ string) (vehicleSession, error) {
				if stage == "lookup" || stage == "lookup_success" {
					if err := block(ctx); err != nil {
						return nil, err
					}
				}
				return v, nil
			}
			if stage == "connect" || stage == "connect_success" {
				v.connect = block
			}
			if stage == "session" || stage == "session_success" {
				v.session = block
			}
			defer func() {
				select {
				case <-release:
				default:
					close(release)
				}
			}()
			done := make(chan struct{})
			var rec *httptest.ResponseRecorder
			go func() {
				defer close(done)
				rec = serveCancellationCommand(ctx, p, testVIN)
			}()
			t.Cleanup(func() { waitCancellationSignal(t, done) })
			waitCancellationSignal(t, entered)
			cancel()
			close(release)
			waitCancellationSignal(t, done)
			if err := <-observed; !errors.Is(err, context.Canceled) {
				t.Errorf("%s context error=%v, want context.Canceled", stage, err)
			}
			if rec.Code != http.StatusInternalServerError || v.executions != 0 {
				t.Errorf("status=%d executions=%d, want unchanged 500 mapping and no command", rec.Code, v.executions)
			}
			wantDisconnects := 1
			if stage == "lookup" {
				wantDisconnects = 0
			}
			if v.disconnects != wantDisconnects {
				t.Errorf("disconnects=%d, want %d", v.disconnects, wantDisconnects)
			}
			wantConnects, wantSessions := 1, 0
			if stage == "lookup" || stage == "lookup_success" {
				wantConnects = 0
			}
			if stage == "session" || stage == "session_success" {
				wantSessions = 1
			}
			if v.connects != wantConnects || v.sessions != wantSessions {
				t.Errorf("connects=%d sessions=%d, want %d and %d", v.connects, v.sessions, wantConnects, wantSessions)
			}
			wantCacheUpdates := 0
			if stage == "session_success" {
				wantCacheUpdates = 1
			}
			if v.cacheUpdates != wantCacheUpdates {
				t.Errorf("cache updates=%d, want %d", v.cacheUpdates, wantCacheUpdates)
			}
			if _, locked := p.vinLock.Load(testVIN); locked {
				t.Fatal("canceled setup retained the VIN lock")
			}
		})
	}
}

func TestVehicleCommandCleanupLifecycle(t *testing.T) {
	for _, stage := range []string{"connect_error", "session_error", "success"} {
		t.Run(stage, func(t *testing.T) {
			p := newTestProxy(t)
			v := &cancellationVehicle{}
			fail := func(context.Context) error { return errors.New("setup failed") }
			if stage == "connect_error" {
				v.connect = fail
			} else if stage == "session_error" {
				v.session = fail
			}
			p.fetchVehicle = func(context.Context, *account.Account, string) (vehicleSession, error) { return v, nil }
			rec := serveCancellationCommand(context.Background(), p, testVIN)
			if v.disconnects != 1 {
				t.Fatalf("disconnects=%d, want exactly one", v.disconnects)
			}
			if stage == "success" {
				if rec.Code != http.StatusOK || v.executions != 1 || v.cacheUpdates != 1 {
					t.Fatalf("status=%d executions=%d cache updates=%d", rec.Code, v.executions, v.cacheUpdates)
				}
				if len(v.cleanupEvents) != 2 || v.cleanupEvents[0] != "cache" || v.cleanupEvents[1] != "disconnect" {
					t.Fatalf("cleanup order=%v, want cache before disconnect", v.cleanupEvents)
				}
			} else if rec.Code != http.StatusInternalServerError || v.executions != 0 || v.cacheUpdates != 0 {
				t.Fatalf("failed setup: status=%d executions=%d cache updates=%d", rec.Code, v.executions, v.cacheUpdates)
			}
		})
	}
}

func TestVehicleCommandCancellationBeforeUnsupportedFallback(t *testing.T) {
	p := newTestProxy(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	v := &cancellationVehicle{session: func(context.Context) error {
		cancel()
		return protocol.ErrProtocolNotSupported
	}}
	p.fetchVehicle = func(context.Context, *account.Account, string) (vehicleSession, error) { return v, nil }
	forwarded := 0
	p.client = roundTripFunc(func(*http.Request) (*http.Response, error) {
		forwarded++
		return jsonResponse(http.StatusOK, `{}`, nil), nil
	})
	rec := serveCancellationCommand(ctx, p, testVIN)
	if rec.Code != http.StatusInternalServerError || forwarded != 0 || v.executions != 0 || v.disconnects != 1 {
		t.Fatalf("status=%d forwarded=%d executions=%d disconnects=%d, want 500, no dispatch, and one cleanup", rec.Code, forwarded, v.executions, v.disconnects)
	}
	if !p.isNotSupported(testVIN) {
		t.Fatal("unsupported protocol fact was not retained")
	}
}

func TestVehicleCommandCancellationDeadline(t *testing.T) {
	t.Run("earlier_parent_deadline", func(t *testing.T) {
		p := newTestProxy(t)
		p.Timeout = time.Minute
		deadline := time.Now().Add(10 * time.Second)
		ctx, cancel := context.WithDeadline(context.Background(), deadline)
		defer cancel()
		p.fetchVehicle = func(ctx context.Context, _ *account.Account, _ string) (vehicleSession, error) {
			if got, ok := ctx.Deadline(); !ok || !got.Equal(deadline) {
				t.Errorf("lookup deadline=%v, want parent %v", got, deadline)
			}
			return &cancellationVehicle{}, nil
		}
		if rec := serveCancellationCommand(ctx, p, testVIN); rec.Code != http.StatusOK {
			t.Fatalf("status=%d", rec.Code)
		}
	})
	for _, stage := range []string{"lookup", "connect", "session"} {
		t.Run("proxy_timeout_"+stage, func(t *testing.T) {
			p := newTestProxy(t)
			p.Timeout = 10 * time.Millisecond
			v := &cancellationVehicle{}
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			// Capture the outer deadline separately from the per-request context.
			parentDeadline, _ := ctx.Deadline()
			block := func(ctx context.Context) error {
				if got, ok := ctx.Deadline(); !ok || !got.Before(parentDeadline) {
					t.Error("proxy did not shorten the parent deadline")
				}
				select {
				case <-ctx.Done():
				case <-time.After(5 * time.Second):
					return errors.New("proxy timeout did not cancel setup")
				}
				if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
					t.Errorf("setup error=%v, want deadline exceeded", ctx.Err())
				}
				return ctx.Err()
			}
			p.fetchVehicle = func(ctx context.Context, _ *account.Account, _ string) (vehicleSession, error) {
				if stage == "lookup" {
					return nil, block(ctx)
				}
				return v, nil
			}
			if stage == "connect" {
				v.connect = block
			} else if stage == "session" {
				v.session = block
			}
			rec := serveCancellationCommand(ctx, p, testVIN)
			if rec.Code != http.StatusInternalServerError || v.executions != 0 {
				t.Fatalf("status=%d executions=%d", rec.Code, v.executions)
			}
			wantDisconnects := 1
			if stage == "lookup" {
				wantDisconnects = 0
			}
			if v.disconnects != wantDisconnects {
				t.Fatalf("disconnects=%d, want %d", v.disconnects, wantDisconnects)
			}
		})
	}
}

func TestVehicleCommandCancellationWhileWaitingForVIN(t *testing.T) {
	p := newTestProxy(t)
	p.Timeout = 30 * time.Second
	if err := p.lockVIN(context.Background(), testVIN); err != nil {
		t.Fatal(err)
	}
	ownerLocked := true
	defer func() {
		if ownerLocked {
			p.unlockVIN(testVIN)
		}
	}()
	lookups := 0
	p.fetchVehicle = func(context.Context, *account.Account, string) (vehicleSession, error) {
		lookups++
		return &cancellationVehicle{}, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started, done := make(chan struct{}), make(chan struct{})
	var rec *httptest.ResponseRecorder
	go func() {
		defer close(done)
		close(started)
		rec = serveCancellationCommand(ctx, p, testVIN)
	}()
	t.Cleanup(func() { waitCancellationSignal(t, done) })
	waitCancellationSignal(t, started)
	cancel()
	waitCancellationSignal(t, done)
	if rec.Code != http.StatusServiceUnavailable || lookups != 0 {
		t.Fatalf("status=%d lookups=%d, want 503 and no lookup", rec.Code, lookups)
	}
	if _, locked := p.vinLock.Load(testVIN); !locked {
		t.Fatal("canceled waiter released another request's VIN lock")
	}
	p.unlockVIN(testVIN)
	ownerLocked = false
	if rec := serveCancellationCommand(context.Background(), p, testVIN); rec.Code != http.StatusOK || lookups != 1 {
		t.Fatalf("next request: status=%d lookups=%d, want 200 and one lookup", rec.Code, lookups)
	}
}

func TestVehicleCommandCancellationKeepsOtherVINIndependent(t *testing.T) {
	p := newTestProxy(t)
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	first, second := &cancellationVehicle{}, &cancellationVehicle{}
	first.session = func(ctx context.Context) error {
		close(entered)
		<-release
		return ctx.Err()
	}
	p.fetchVehicle = func(_ context.Context, _ *account.Account, vin string) (vehicleSession, error) {
		if vin == testVIN {
			return first, nil
		}
		return second, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	go func() {
		defer close(done)
		serveCancellationCommand(ctx, p, testVIN)
	}()
	t.Cleanup(func() { waitCancellationSignal(t, done) })
	waitCancellationSignal(t, entered)
	if rec := serveCancellationCommand(context.Background(), p, "5YJ3E1EA1KF000002"); rec.Code != http.StatusOK {
		t.Fatalf("independent VIN status=%d", rec.Code)
	}
	cancel()
	close(release)
	waitCancellationSignal(t, done)
	if first.executions != 0 || first.disconnects != 1 || second.executions != 1 || second.disconnects != 1 {
		t.Fatalf("first executions=%d disconnects=%d; second executions=%d disconnects=%d", first.executions, first.disconnects, second.executions, second.disconnects)
	}
}
