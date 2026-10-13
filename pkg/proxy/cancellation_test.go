package proxy

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestForwardRequestCancellationBoundaries(t *testing.T) {
	t.Run("cancel_after_redirect_before_retry", func(t *testing.T) {
		p := newTestProxy(t)
		p.Timeout = 2 * time.Second
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		attempts := 0
		p.client = roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			attempts++
			if attempts == 1 {
				cancel()
				header := make(http.Header)
				header.Set("Alt-Svc", h2Prefix+"alt.example.tesla.com")
				return jsonResponse(http.StatusMisdirectedRequest, `{}`, header), nil
			}
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		})
		req := httptest.NewRequest(http.MethodGet, "/api/1/vehicles", nil).WithContext(ctx)
		req.Header.Set("Authorization", authHeader())
		rec := httptest.NewRecorder()
		p.ServeHTTP(rec, req)
		if attempts != 1 {
			t.Fatalf("upstream attempts=%d, want1 after parent cancellation", attempts)
		}
		if rec.Code != http.StatusGatewayTimeout {
			t.Fatalf("status=%d, want504 in current retry-cancellation mapping", rec.Code)
		}
	})
	t.Run("shorter_parent_deadline", func(t *testing.T) {
		p := newTestProxy(t)
		p.Timeout = 20 * time.Second
		deadline := time.Now().Add(5 * time.Second)
		ctx, cancel := context.WithDeadline(context.Background(), deadline)
		defer cancel()
		p.client = roundTripFunc(func(req *http.Request) (*http.Response, error) {
			got, ok := req.Context().Deadline()
			if !ok || !got.Equal(deadline) {
				t.Errorf("forwarded deadline=%v, want parent %v", got, deadline)
			}
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		})
		req := httptest.NewRequest(http.MethodGet, "/api/1/vehicles", nil).WithContext(ctx)
		req.Header.Set("Authorization", authHeader())
		p.ServeHTTP(httptest.NewRecorder(), req)
	})
	t.Run("proxy_timeout_still_bounds_parent", func(t *testing.T) {
		p := newTestProxy(t)
		p.Timeout = 50 * time.Millisecond
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		p.client = roundTripFunc(func(req *http.Request) (*http.Response, error) {
			parentDeadline, _ := ctx.Deadline()
			forwardedDeadline, ok := req.Context().Deadline()
			if !ok || !forwardedDeadline.Before(parentDeadline) {
				t.Errorf("proxy timeout did not shorten parent deadline")
			}
			select {
			case <-req.Context().Done():
			case <-time.After(2 * time.Second):
				t.Fatal("forwarded request did not observe its timeout")
			}
			if !errors.Is(req.Context().Err(), context.DeadlineExceeded) {
				t.Errorf("error=%v", req.Context().Err())
			}
			return nil, req.Context().Err()
		})
		req := httptest.NewRequest(http.MethodGet, "/api/1/vehicles", nil).WithContext(ctx)
		req.Header.Set("Authorization", authHeader())
		p.ServeHTTP(httptest.NewRecorder(), req)
	})
	t.Run("normal_success", func(t *testing.T) {
		p := newTestProxy(t)
		attempts := 0
		p.client = roundTripFunc(func(req *http.Request) (*http.Response, error) {
			attempts++
			if req.Context().Err() != nil {
				t.Error(req.Context().Err())
			}
			return jsonResponse(http.StatusOK, `{"ok":true}`, nil), nil
		})
		req := httptest.NewRequest(http.MethodGet, "/api/1/vehicles", nil)
		req.Header.Set("Authorization", authHeader())
		rec := httptest.NewRecorder()
		p.ServeHTTP(rec, req)
		if attempts != 1 || rec.Code != http.StatusOK || rec.Body.String() != `{"ok":true}` {
			t.Fatalf("attempts=%d status=%d body=%s", attempts, rec.Code, rec.Body.String())
		}
	})
}
