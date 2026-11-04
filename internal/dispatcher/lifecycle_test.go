package dispatcher

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/teslamotors/vehicle-command/pkg/connector"
	"github.com/teslamotors/vehicle-command/pkg/protocol"
)

// lifecycleConnector gates Receive so startup cancellation does not depend on
// which goroutine the scheduler runs first. It never sends anything externally.
type lifecycleConnector struct {
	inbox        chan []byte
	entered      chan struct{}
	release      chan struct{}
	enterOnce    sync.Once
	releaseOnce  sync.Once
	closeOnce    sync.Once
	receiveCalls atomic.Int32
}

func (c *lifecycleConnector) Receive() <-chan []byte {
	c.receiveCalls.Add(1)
	c.enterOnce.Do(func() { close(c.entered) })
	<-c.release
	return c.inbox
}

func (c *lifecycleConnector) unblock()                           { c.releaseOnce.Do(func() { close(c.release) }) }
func (c *lifecycleConnector) Close()                             { c.closeOnce.Do(func() { close(c.inbox) }) }
func (c *lifecycleConnector) Send(context.Context, []byte) error { return nil }
func (c *lifecycleConnector) VIN() string                        { return "0123456789ABCDEFG" }
func (c *lifecycleConnector) PreferredAuthMethod() connector.AuthMethod {
	return connector.AuthMethodNone
}
func (c *lifecycleConnector) RetryInterval() time.Duration  { return time.Millisecond }
func (c *lifecycleConnector) AllowedLatency() time.Duration { return time.Second }

func newLifecycleDispatcher(t *testing.T) (*Dispatcher, *lifecycleConnector) {
	t.Helper()
	c := &lifecycleConnector{
		inbox:   make(chan []byte),
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	d, err := New(c, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		c.unblock()
		d.Stop()
		c.Close()
	})
	return d, c
}

func waitLifecycle(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for dispatcher lifecycle")
	}
}

func lifecycleResult(t *testing.T, results <-chan error) error {
	t.Helper()
	select {
	case err := <-results:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("dispatcher startup did not finish")
		return nil
	}
}

func assertLifecycleDisconnected(t *testing.T, d *Dispatcher) {
	t.Helper()
	recv, err := d.Send(context.Background(), testCommand(), connector.AuthMethodNone)
	if recv != nil {
		recv.Close()
	}
	if !errors.Is(err, protocol.ErrNotConnected) {
		t.Fatalf("Send after failed or stopped startup = %v, want ErrNotConnected", err)
	}
}

func TestLifecycleStartAlreadyCanceled(t *testing.T) {
	d, c := newLifecycleDispatcher(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := d.Start(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Start = %v, want context.Canceled", err)
	}
	if calls := c.receiveCalls.Load(); calls != 0 {
		t.Fatalf("Receive calls = %d, want no listener for canceled startup", calls)
	}
	assertLifecycleDisconnected(t, d)
}

func TestLifecycleCancelDuringStartup(t *testing.T) {
	d, c := newLifecycleDispatcher(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- d.Start(ctx) }()
	waitLifecycle(t, c.entered)
	cancel()
	c.unblock()
	if err := lifecycleResult(t, result); !errors.Is(err, context.Canceled) {
		t.Fatalf("Start = %v, want context.Canceled", err)
	}
	assertLifecycleDisconnected(t, d)

	// Failed startup must leave no listener behind, so the same dispatcher can
	// start again without waiting for the old context or closing the connector.
	retryCtx, retryCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer retryCancel()
	if err := d.Start(retryCtx); err != nil {
		t.Fatalf("restart after canceled startup: %v", err)
	}
}

func TestLifecycleClosedBeforeReady(t *testing.T) {
	d, c := newLifecycleDispatcher(t)
	c.Close()
	c.unblock()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := d.Start(ctx); !errors.Is(err, protocol.ErrNotConnected) {
		t.Fatalf("Start with closed connector = %v, want ErrNotConnected", err)
	}
	assertLifecycleDisconnected(t, d)
}

func TestLifecycleStopDuringStartup(t *testing.T) {
	d, c := newLifecycleDispatcher(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- d.Start(ctx) }()
	waitLifecycle(t, c.entered)
	stopped := make(chan struct{})
	go func() {
		d.Stop()
		close(stopped)
	}()
	c.unblock()
	waitLifecycle(t, stopped)
	// Startup may become ready before Stop, or Stop may win. Neither ordering
	// should leave Start waiting for its context or leave a listener running.
	if err := lifecycleResult(t, result); err != nil && !errors.Is(err, protocol.ErrNotConnected) {
		t.Fatalf("Start racing with Stop = %v", err)
	}
	assertLifecycleDisconnected(t, d)
}

type lifecycleObservedContext struct {
	context.Context
	observed chan struct{}
	once     sync.Once
}

func (c *lifecycleObservedContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.observed) })
	return c.Context.Done()
}

func TestLifecycleCanceledConcurrentStartKeepsOwner(t *testing.T) {
	d, c := newLifecycleDispatcher(t)
	ownerCtx, ownerCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer ownerCancel()
	ownerResult := make(chan error, 1)
	go func() { ownerResult <- d.Start(ownerCtx) }()
	waitLifecycle(t, c.entered)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	observed := &lifecycleObservedContext{Context: ctx, observed: make(chan struct{})}
	otherResult := make(chan error, 1)
	go func() { otherResult <- d.Start(observed) }()
	waitLifecycle(t, observed.observed)
	cancel()
	if err := lifecycleResult(t, otherResult); !errors.Is(err, context.Canceled) {
		t.Fatalf("concurrent Start = %v, want context.Canceled", err)
	}

	c.unblock()
	if err := lifecycleResult(t, ownerResult); err != nil {
		t.Fatalf("original Start interrupted by another caller: %v", err)
	}
	if calls := c.receiveCalls.Load(); calls != 1 {
		t.Fatalf("Receive calls = %d, want one shared listener", calls)
	}
}

func TestLifecycleStopAndRestart(t *testing.T) {
	d, c := newLifecycleDispatcher(t)
	c.unblock()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := d.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if err := d.Start(ctx); err != nil {
		t.Fatalf("Start of ready dispatcher: %v", err)
	}

	const stoppers = 4
	var stopped sync.WaitGroup
	stopped.Add(stoppers)
	for i := 0; i < stoppers; i++ {
		go func() {
			defer stopped.Done()
			d.Stop()
		}()
	}
	done := make(chan struct{})
	go func() { stopped.Wait(); close(done) }()
	waitLifecycle(t, done)
	assertLifecycleDisconnected(t, d)
	if err := d.Start(ctx); err != nil {
		t.Fatalf("Start after concurrent Stop calls: %v", err)
	}
	if calls := c.receiveCalls.Load(); calls != 2 {
		t.Fatalf("Receive calls = %d, want one listener per startup", calls)
	}
}
