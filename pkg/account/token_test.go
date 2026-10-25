package account

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestExchangeSendsVehicleLocationScope(t *testing.T) {
	var gotScope string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		values, err := url.ParseQuery(string(body))
		if err != nil {
			t.Errorf("parse form: %v", err)
		}
		if values.Get("grant_type") != GrantAuthorizationCode {
			t.Errorf("grant_type = %q", values.Get("grant_type"))
		}
		gotScope = values.Get("scope")
		if !HasScope(strings.Fields(gotScope), ScopeVehicleLocation) {
			t.Errorf("token exchange omitted vehicle_location: %q", gotScope)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"access_token":  makeScopedJWT(DefaultThirdPartyScopes),
			"refresh_token": "refresh",
			"token_type":    "Bearer",
		})
	}))
	defer server.Close()

	resp, err := ExchangeAuthorizationCode(context.Background(), server.Client(), &TokenRequest{
		ClientID:     "client",
		ClientSecret: "secret",
		Code:         "code",
		Audience:     "https://fleet-api.prd.na.vn.cloud.tesla.com",
		RedirectURI:  "https://example.com/callback",
		TokenURL:     server.URL,
	})
	if err != nil {
		t.Fatalf("ExchangeAuthorizationCode: %v", err)
	}
	if !HasScope(resp.Scopes, ScopeVehicleLocation) {
		t.Fatalf("issued token scopes = %v, want vehicle_location", resp.Scopes)
	}
	if !strings.Contains(gotScope, ScopeVehicleLocation) {
		t.Fatalf("POST scope = %q, want vehicle_location", gotScope)
	}
}

func TestRefreshSendsVehicleLocationScope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "scope=") || !strings.Contains(string(body), ScopeVehicleLocation) {
			t.Errorf("refresh omitted vehicle_location: %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"access_token":  makeScopedJWT(DefaultThirdPartyScopes),
			"refresh_token": "next",
		})
	}))
	defer server.Close()

	resp, err := RefreshAccessToken(context.Background(), server.Client(), &TokenRequest{
		ClientID:     "client",
		RefreshToken: "old",
		TokenURL:     server.URL,
	})
	if err != nil {
		t.Fatalf("RefreshAccessToken: %v", err)
	}
	if resp.RefreshToken != "next" {
		t.Fatalf("refresh_token = %q", resp.RefreshToken)
	}
}

func TestExchangeReportsDroppedVehicleLocation(t *testing.T) {
	granted := []string{ScopeOpenID, ScopeVehicleDeviceData, ScopeVehicleCmds}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"access_token":  makeScopedJWT(granted),
			"refresh_token": "refresh",
		})
	}))
	defer server.Close()

	resp, err := Exchange(context.Background(), server.Client(), &TokenRequest{
		GrantType:    GrantAuthorizationCode,
		ClientID:     "client",
		ClientSecret: "secret",
		Code:         "code",
		Audience:     "https://fleet-api.prd.na.vn.cloud.tesla.com",
		RedirectURI:  "https://example.com/callback",
		TokenURL:     server.URL,
	})
	if resp == nil || resp.RefreshToken != "refresh" {
		t.Fatalf("expected tokens despite missing scope, got %+v", resp)
	}
	if !errors.Is(err, ErrMissingVehicleLocation) {
		t.Fatalf("err = %v, want ErrMissingVehicleLocation", err)
	}
	var missing *MissingScopeError
	if !errors.As(err, &missing) {
		t.Fatalf("err = %v, want MissingScopeError", err)
	}
	if !HasScope(missing.Missing, ScopeVehicleLocation) {
		t.Fatalf("missing = %v, want vehicle_location", missing.Missing)
	}
}

func TestExchangeRejectsAuthAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"code expired"}`))
	}))
	defer server.Close()

	_, err := ExchangeAuthorizationCode(context.Background(), server.Client(), &TokenRequest{
		ClientID:     "client",
		ClientSecret: "secret",
		Code:         "stale",
		Audience:     "https://fleet-api.prd.na.vn.cloud.tesla.com",
		RedirectURI:  "https://example.com/callback",
		TokenURL:     server.URL,
	})
	if err == nil || !strings.Contains(err.Error(), "invalid_grant") {
		t.Fatalf("err = %v, want invalid_grant", err)
	}
}

func TestExchangeCustomScopesSkipVehicleLocation(t *testing.T) {
	scopes := []string{ScopeOpenID, ScopeVehicleCmds}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		values, _ := url.ParseQuery(string(body))
		if strings.Contains(values.Get("scope"), ScopeVehicleLocation) {
			t.Errorf("custom scopes should not add vehicle_location: %q", values.Get("scope"))
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"access_token": makeScopedJWT(scopes),
		})
	}))
	defer server.Close()

	resp, err := Exchange(context.Background(), server.Client(), &TokenRequest{
		GrantType:    GrantRefreshToken,
		ClientID:     "client",
		RefreshToken: "old",
		Scopes:       scopes,
		TokenURL:     server.URL,
	})
	if err != nil {
		t.Fatalf("custom scopes: %v", err)
	}
	if HasScope(resp.Scopes, ScopeVehicleLocation) {
		t.Fatal("did not request vehicle_location")
	}
}

func TestTokenRequestValidation(t *testing.T) {
	_, err := Exchange(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
	_, err = ExchangeAuthorizationCode(context.Background(), nil, &TokenRequest{ClientID: "id"})
	if err == nil {
		t.Fatal("expected error for incomplete authorization_code request")
	}
	_, err = RefreshAccessToken(context.Background(), nil, &TokenRequest{ClientID: "id"})
	if err == nil {
		t.Fatal("expected error for incomplete refresh request")
	}
}

func TestAccountHasScope(t *testing.T) {
	payload := &oauthPayload{
		Audiences: []string{"https://fleet-api.prd.na.vn.cloud.tesla.com"},
		Scopes:    []string{ScopeVehicleDeviceData, ScopeVehicleLocation},
		Subject:   "user",
	}
	acct, err := New(makeTestJWT(payload), "")
	if err != nil {
		t.Fatal(err)
	}
	if !acct.HasScope(ScopeVehicleLocation) {
		t.Fatal("expected vehicle_location on account")
	}
	if acct.HasScope(ScopeVehicleCmds) {
		t.Fatal("did not expect vehicle_cmds")
	}
}

func makeScopedJWT(scopes []string) string {
	return makeTestJWT(&oauthPayload{
		Audiences: []string{"https://fleet-api.prd.na.vn.cloud.tesla.com"},
		Scopes:    scopes,
	})
}
