package main

import (
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestRequestIDInjected(t *testing.T) {
    h := withRequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    }))

    req := httptest.NewRequest("GET", "/", nil)
    rr := httptest.NewRecorder()
    h.ServeHTTP(rr, req)

    if rr.Header().Get("X-Request-Id") == "" {
        t.Fatal("expected X-Request-Id header")
    }
}
