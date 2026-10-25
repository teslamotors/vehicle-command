package account

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/teslamotors/vehicle-command/pkg/connector"
)

// DefaultTokenURL is Tesla's Fleet Auth token endpoint. Server-to-server
// exchanges must use this host, not auth.tesla.com.
const DefaultTokenURL = "https://fleet-auth.prd.vn.cloud.tesla.com/oauth2/v3/token"

const (
	GrantAuthorizationCode = "authorization_code"
	GrantRefreshToken      = "refresh_token"
)

// ErrMissingVehicleLocation is returned when a requested vehicle_location
// scope is absent from the issued access token.
var ErrMissingVehicleLocation = errors.New("access token missing vehicle_location scope")

// TokenRequest is a Fleet Auth token exchange.
type TokenRequest struct {
	GrantType    string
	ClientID     string
	ClientSecret string
	Code         string
	RefreshToken string
	Audience     string
	RedirectURI  string
	Scopes       []string
	TokenURL     string
}

func (r *TokenRequest) scopes() []string {
	if r != nil && len(r.Scopes) > 0 {
		out := make([]string, len(r.Scopes))
		copy(out, r.Scopes)
		return out
	}
	out := make([]string, len(DefaultThirdPartyScopes))
	copy(out, DefaultThirdPartyScopes)
	return out
}

// TokenResponse is a successful Fleet Auth token payload.
type TokenResponse struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	IDToken      string   `json:"id_token"`
	TokenType    string   `json:"token_type"`
	ExpiresIn    int      `json:"expires_in"`
	Scopes       []string `json:"-"`
}

// MissingScopeError is returned when Tesla issues a token that omits requested
// resource scopes. Response is still populated so a rotated refresh token can
// be saved.
type MissingScopeError struct {
	Missing  []string
	Granted  []string
	Response *TokenResponse
}

func (e *MissingScopeError) Error() string {
	return fmt.Sprintf("token missing scopes %s (granted: %s)", strings.Join(e.Missing, " "), strings.Join(e.Granted, " "))
}

func (e *MissingScopeError) Is(target error) bool {
	return target == ErrMissingVehicleLocation && e != nil && HasScope(e.Missing, ScopeVehicleLocation)
}

// TokenScopes returns the scp claim from an access token.
func TokenScopes(accessToken string) ([]string, error) {
	payload, err := decodeOAuthPayload(accessToken)
	if err != nil {
		return nil, err
	}
	return payload.Scopes, nil
}

// ExchangeAuthorizationCode exchanges an authorization code for tokens.
func ExchangeAuthorizationCode(ctx context.Context, client *http.Client, req *TokenRequest) (*TokenResponse, error) {
	if req != nil {
		req.GrantType = GrantAuthorizationCode
	}
	return Exchange(ctx, client, req)
}

// RefreshAccessToken exchanges a refresh token for a new token pair.
func RefreshAccessToken(ctx context.Context, client *http.Client, req *TokenRequest) (*TokenResponse, error) {
	if req != nil {
		req.GrantType = GrantRefreshToken
	}
	return Exchange(ctx, client, req)
}

// Exchange performs a Fleet Auth token request. The scope form field always
// includes vehicle_location unless the caller supplies a custom list. If Tesla
// drops a requested resource scope, Exchange returns the tokens and a
// [MissingScopeError].
func Exchange(ctx context.Context, client *http.Client, req *TokenRequest) (*TokenResponse, error) {
	form, endpoint, err := req.encode()
	if err != nil {
		return nil, err
	}
	if client == nil {
		client = http.DefaultClient
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(io.LimitReader(resp.Body, connector.MaxResponseLength+1))
	if err != nil {
		return nil, err
	}
	if len(body) > connector.MaxResponseLength {
		return nil, errors.New("token response exceeds maximum length")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, tokenExchangeError(body)
	}

	var tokens TokenResponse
	if err := json.Unmarshal(body, &tokens); err != nil {
		return nil, fmt.Errorf("token exchange returned malformed JSON: %w", err)
	}
	if tokens.AccessToken == "" {
		return nil, errors.New("token exchange returned an empty access_token")
	}

	granted, err := TokenScopes(tokens.AccessToken)
	if err != nil {
		return nil, err
	}
	tokens.Scopes = granted

	if missing := missingResourceScopes(req.scopes(), granted); len(missing) > 0 {
		return &tokens, &MissingScopeError{Missing: missing, Granted: granted, Response: &tokens}
	}
	return &tokens, nil
}

func (r *TokenRequest) encode() (url.Values, string, error) {
	if r == nil {
		return nil, "", errors.New("token request is required")
	}
	if r.ClientID == "" {
		return nil, "", errors.New("client_id is required")
	}
	if r.GrantType == "" {
		return nil, "", errors.New("grant_type is required")
	}

	form := url.Values{}
	form.Set("grant_type", r.GrantType)
	form.Set("client_id", r.ClientID)
	form.Set("scope", strings.Join(r.scopes(), " "))

	switch r.GrantType {
	case GrantAuthorizationCode:
		if r.Code == "" || r.ClientSecret == "" || r.Audience == "" || r.RedirectURI == "" {
			return nil, "", errors.New("authorization_code grant requires code, client_secret, audience, and redirect_uri")
		}
		form.Set("client_secret", r.ClientSecret)
		form.Set("code", r.Code)
		form.Set("audience", r.Audience)
		form.Set("redirect_uri", r.RedirectURI)
	case GrantRefreshToken:
		if r.RefreshToken == "" {
			return nil, "", errors.New("refresh_token grant requires refresh_token")
		}
		form.Set("refresh_token", r.RefreshToken)
		if r.ClientSecret != "" {
			form.Set("client_secret", r.ClientSecret)
		}
		if r.Audience != "" {
			form.Set("audience", r.Audience)
		}
	default:
		return nil, "", fmt.Errorf("unsupported grant_type %s", r.GrantType)
	}

	endpoint := r.TokenURL
	if endpoint == "" {
		endpoint = DefaultTokenURL
	}
	return form, endpoint, nil
}

func tokenExchangeError(body []byte) error {
	var apiErr struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if json.Unmarshal(body, &apiErr) == nil && apiErr.Error != "" {
		if apiErr.ErrorDescription != "" {
			return fmt.Errorf("token exchange failed: %s (%s)", apiErr.Error, apiErr.ErrorDescription)
		}
		return fmt.Errorf("token exchange failed: %s", apiErr.Error)
	}
	msg := strings.TrimSpace(string(body))
	if msg == "" {
		return errors.New("token exchange failed")
	}
	return fmt.Errorf("token exchange failed: %s", msg)
}

func missingResourceScopes(requested, granted []string) []string {
	var missing []string
	for _, scope := range requested {
		if scope == ScopeOpenID || scope == ScopeOfflineAccess {
			continue
		}
		if !HasScope(granted, scope) {
			missing = append(missing, scope)
		}
	}
	return missing
}
