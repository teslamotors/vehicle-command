package proxy

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

// b64Encode encodes a string to base64 without padding.
func b64Encode(payload string) string {
	return base64.RawStdEncoding.EncodeToString([]byte(payload))
}

type oauthPayload struct {
	Audiences []string `json:"aud"`
	OUCode    string   `json:"ou_code"`
}

// makeTestJWT creates a JWT string with the given payload.
func makeTestJWT(payload *oauthPayload) string {
	jwtBody, _ := json.Marshal(payload)
	return fmt.Sprintf("x.%s.y", b64Encode(string(jwtBody)))
}

var (
	payload = &oauthPayload{
		Audiences: []string{
			"https://auth.tesla.com/nts",
			"https://fleet-api.prd.na.vn.cloud.tesla.com",
			"https://fleet-api.prd.eu.vn.cloud.tesla.com",
		},
		OUCode: "EU",
	}
)

func TestGetAccount(t *testing.T) {
	Header := map[string][]string{
		"Authorization": {"Bearer " + makeTestJWT(payload)},
	}
	req := &http.Request{Header: Header}

	acct, _ := getAccount(req, "")
	expectedHost := "fleet-api.prd.eu.vn.cloud.tesla.com"

	if acct == nil || acct.Host != expectedHost {
		t.Errorf("acct = %+v, expected Host = %s", acct, expectedHost)
	}
}

func TestGetAccountWithConfigOverride(t *testing.T) {
	Header := map[string][]string{
		"Authorization": {"Bearer " + makeTestJWT(payload)},
	}
	req := &http.Request{Header: Header}

	acct, _ := getAccount(req, "fleet-api.prd.na.vn.cloud.tesla.com")
	expectedHost := "fleet-api.prd.na.vn.cloud.tesla.com"

	if acct == nil || acct.Host != expectedHost {
		t.Errorf("acct = %+v, expected Host = %s", acct, expectedHost)
	}
}

func TestGetAccountWithHeaderOverride(t *testing.T) {
	Header := map[string][]string{
		"Authorization": {"Bearer " + makeTestJWT(payload)},
		"Fleetapi-Host": {"fleet-api.prd.na.vn.cloud.tesla.com"},
	}
	req := &http.Request{Header: Header}

	acct, _ := getAccount(req, "")
	expectedHost := "fleet-api.prd.na.vn.cloud.tesla.com"

	if acct == nil || acct.Host != expectedHost {
		t.Errorf("acct = %+v, expected Host = %s", acct, expectedHost)
	}
}

func TestGetAccountWithConfigAndHeaderOverride(t *testing.T) {
	Header := map[string][]string{
		"Authorization": {"Bearer " + makeTestJWT(payload)},
		"Fleetapi-Host": {"fleet-api.prd.na.vn.cloud.tesla.com"},
	}
	req := &http.Request{Header: Header}

	acct, _ := getAccount(req, "fleet-api.prd.na.vn.cloud.tesla.com")
	expectedHost := "fleet-api.prd.na.vn.cloud.tesla.com"

	if acct == nil || acct.Host != expectedHost {
		t.Errorf("acct = %+v, expected Host = %s", acct, expectedHost)
	}
}
