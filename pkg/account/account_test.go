package account

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"
)

// b64Encode encodes a string to base64url without padding.
func b64Encode(payload string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(payload))
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

func TestNewAccountBase64URL(t *testing.T) {
	const domain = "fleet-api.prd.eu.vn.cloud.tesla.com"
	tests := []struct {
		name    string
		subject string
	}{
		{"hyphen", ">>>"},
		{"underscore", "???"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			payload := fmt.Sprintf(`{"aud":["https://%s"],"sub":"%s"}`, domain, test.subject)
			token := "x." + b64Encode(payload) + ".y"
			acct, err := New(token, "")
			if err != nil {
				t.Fatalf("Returned error on valid base64url JWT: %s", err)
			}
			if acct.Host != domain || acct.Subject != test.subject {
				t.Errorf("acct = %+v, expected Host = %s and Subject = %s", acct, domain, test.subject)
			}
			if acct.authHeader != "Bearer "+token {
				t.Errorf("Authorization header does not preserve the original JWT")
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
