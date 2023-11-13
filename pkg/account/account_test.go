package account

import (
	"encoding/base64"
	"fmt"
	"testing"
)

func b64Encode(payload string) string {
	return base64.RawStdEncoding.EncodeToString([]byte(payload))
}

func TestNewAccount(t *testing.T) {
	validDomain := "fleet-api.example.tesla.com"
	if _, err := New(""); err == nil {
		t.Error("Returned success empty JWT")
	}
	if _, err := New(b64Encode(validDomain)); err == nil {
		t.Error("Returned success on one-field JWT")
	}
	if _, err := New("x." + b64Encode(validDomain)); err == nil {
		t.Error("Returned success on two-field JWT")
	}
	if _, err := New("x." + b64Encode(validDomain) + "y.z"); err == nil {
		t.Error("Returned success on four-field JWT")
	}
	if _, err := New("x." + validDomain + ".y"); err == nil {
		t.Error("Returned success on non-base64 encoded JWT")
	}
	if _, err := New("x." + b64Encode("{\"aud\": \"example.com\"}") + ".y"); err == nil {
		t.Error("Returned success on untrusted domain")
	}
	if _, err := New("x." + b64Encode(fmt.Sprintf("{\"aud\": \"%s\"}", validDomain)) + ".y"); err == nil {
		t.Error("Returned when aud field not a list")
	}

	acct, err := New("x." + b64Encode(fmt.Sprintf("{\"aud\": [\"%s\"]}", validDomain)) + ".y")
	if err != nil {
		t.Fatalf("Returned error on valid JWT: %s", err)
	}
	if acct == nil || acct.Host != validDomain {
		t.Errorf("acct = %+v", acct)
	}
}
