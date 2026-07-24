package main

import (
    "crypto/rand"
    "encoding/hex"
    "net/http"
)

func withRequestID(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        rid := r.Header.Get("X-Request-Id")
        if rid == "" {
            var b [16]byte
            _, _ = rand.Read(b[:])
            rid = hex.EncodeToString(b[:])
        }
        w.Header().Set("X-Request-Id", rid)
        next.ServeHTTP(w, r)
    })
}
