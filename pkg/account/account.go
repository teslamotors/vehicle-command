package account

import (
	"context"
	"crypto/ecdh"
	_ "embed" // Used to embed version for use with user agent
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"runtime/debug"
	"strings"

	"github.com/teslamotors/vehicle-command/internal/authentication"
	"github.com/teslamotors/vehicle-command/internal/log"
	"github.com/teslamotors/vehicle-command/pkg/cache"
	"github.com/teslamotors/vehicle-command/pkg/connector"
	"github.com/teslamotors/vehicle-command/pkg/connector/inet"
	"github.com/teslamotors/vehicle-command/pkg/vehicle"

	"golang.org/x/oauth2"
)

var (
	//go:embed version.txt
	libraryVersion string
)

func buildUserAgent(app string) string {
	library := strings.TrimSpace("tesla-sdk/" + libraryVersion)
	build, ok := debug.ReadBuildInfo()
	if !ok {
		return library
	}
	path := strings.Split(build.Path, "/")
	if len(path) == 0 {
		return library
	}

	if app == "" {
		app = path[len(path)-1]
		var version string
		if build.Main.Version != "(devel)" && build.Main.Version != "" {
			version = build.Main.Version
		} else {
			for _, info := range build.Settings {
				if info.Key == "vcs.revision" {
					if len(info.Value) > 8 {
						version = info.Value[0:8]
					}
					break
				}
			}
		}

		if version != "" {
			app = fmt.Sprintf("%s/%s", app, version)
		}
	}

	return fmt.Sprintf("%s %s", app, library)
}

// Account allows interaction with a Tesla account.
type Account struct {
	// The default UserAgent is constructed from the global UserAgent, but can be overridden.
	UserAgent string
	Host      string
	Subject   string

	// authHeader is used when the Account was created with a static OAuth
	// access token via [New]. When tokenSource is set, authHeader is unused and
	// credentials are obtained from tokenSource on each request.
	authHeader  string
	tokenSource oauth2.TokenSource
	client      http.Client
}

// We don't parse JWTs beyond what's required to extract the API server domain name
type oauthPayload struct {
	Audiences []string `json:"aud"`
	OUCode    string   `json:"ou_code"`
	Subject   string   `json:"sub"`
}

var domainRegEx = regexp.MustCompile(`^[A-Za-z0-9-.]+$`) // We're mostly interested in stopping paths; the http package handles the rest.
var remappedDomains = map[string]string{}                // For use during development; populate in an init() function.

const defaultDomain = "fleet-api.prd.na.vn.cloud.tesla.com"

func (p *oauthPayload) domain() string {
	if len(remappedDomains) > 0 {
		for _, a := range p.Audiences {
			if d, ok := remappedDomains[a]; ok {
				return d
			}
		}
	}
	domain := defaultDomain
	ouCodeMatch := fmt.Sprintf(".%s.", strings.ToLower(p.OUCode))
	for _, u := range p.Audiences {
		if strings.HasPrefix(u, "https://auth.tesla.") {
			continue
		}
		d, _ := strings.CutPrefix(u, "https://")
		d, _ = strings.CutSuffix(d, "/")
		if !domainRegEx.MatchString(d) {
			continue
		}

		if inet.ValidTeslaDomainSuffix(d) && strings.HasPrefix(d, "fleet-api.") {
			domain = d
			// Prefer domains that contain the ou_code (region)
			if strings.Contains(domain, ouCodeMatch) {
				return domain
			}
		}
	}
	return domain
}

// New returns an [Account] that can be used to fetch a [vehicle.Vehicle].
//
// oauthToken is a Fleet API OAuth access token (JWT). The token is stored
// statically on the Account; it is not refreshed. Long-lived applications
// should prefer [FromTokenSource].
//
// Optional userAgent can be passed in - otherwise it will be generated from code.
func New(oauthToken, userAgent string) (*Account, error) {
	payload, err := parseOAuthPayload(oauthToken)
	if err != nil {
		return nil, err
	}
	domain := payload.domain()
	if domain == "" {
		return nil, fmt.Errorf("client provided OAuth token with invalid audiences")
	}
	return &Account{
		UserAgent:  buildUserAgent(userAgent),
		authHeader: "Bearer " + strings.TrimSpace(oauthToken),
		Host:       domain,
		Subject:    payload.Subject,
	}, nil
}

// FromTokenSource returns an [Account] that obtains OAuth credentials from ts
// on each Fleet API request. When ts is a refreshing token source (for example
// from golang.org/x/oauth2), expired access tokens are renewed automatically,
// which is required for long-lived processes.
//
// Domain and subject are taken from the access token returned by the initial
// ts.Token() call. The Account wraps ts with [oauth2.ReuseTokenSource] so
// concurrent callers share cached credentials until they expire.
func FromTokenSource(ts oauth2.TokenSource, userAgent string) (*Account, error) {
	if ts == nil {
		return nil, fmt.Errorf("nil oauth2.TokenSource")
	}
	tok, err := ts.Token()
	if err != nil {
		return nil, fmt.Errorf("oauth2 token source: %w", err)
	}
	if tok == nil || tok.AccessToken == "" {
		return nil, fmt.Errorf("oauth2 token source returned an empty access token")
	}
	payload, err := parseOAuthPayload(tok.AccessToken)
	if err != nil {
		return nil, err
	}
	domain := payload.domain()
	if domain == "" {
		return nil, fmt.Errorf("client provided OAuth token with invalid audiences")
	}
	return &Account{
		UserAgent:   buildUserAgent(userAgent),
		Host:        domain,
		Subject:     payload.Subject,
		tokenSource: oauth2.ReuseTokenSource(tok, ts),
	}, nil
}

// FromToken returns an [Account] backed by tok. If tok includes a refresh token
// and expiry, wrap it with an oauth2.Config TokenSource and use
// [FromTokenSource] instead so access tokens can be refreshed.
func FromToken(tok *oauth2.Token, userAgent string) (*Account, error) {
	if tok == nil {
		return nil, fmt.Errorf("nil oauth2.Token")
	}
	return FromTokenSource(oauth2.StaticTokenSource(tok), userAgent)
}

func parseOAuthPayload(oauthToken string) (*oauthPayload, error) {
	parts := strings.Split(oauthToken, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("client provided malformed OAuth token")
	}
	payloadJSON, err := base64.RawStdEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("client provided malformed OAuth token: %s (%s)", err, parts[1])
	}
	var payload oauthPayload
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return nil, fmt.Errorf("client provided malformed OAuth token: %s", err)
	}
	return &payload, nil
}

func (a *Account) authorization() (string, error) {
	if a.tokenSource != nil {
		tok, err := a.tokenSource.Token()
		if err != nil {
			return "", fmt.Errorf("oauth2 token source: %w", err)
		}
		if tok == nil || tok.AccessToken == "" {
			return "", fmt.Errorf("oauth2 token source returned an empty access token")
		}
		tokenType := strings.TrimSpace(tok.Type())
		if tokenType == "" {
			tokenType = "Bearer"
		}
		return tokenType + " " + tok.AccessToken, nil
	}
	return a.authHeader, nil
}

// GetVehicle returns the Vehicle belonging to the account with the provided vin.
//
// Providing a nil privateKey is allowed, but a privateKey is required for most Vehicle
// interactions. Typically, the privateKey will only be nil when connecting to the Vehicle to send
// an AddKeyRequest; see documentation in [pkg/github.com/teslamotors/vehicle-command/pkg/vehicle]. The
// sessions parameter may also be nil, but providing a cache.SessionCache avoids a round-trip
// handshake with the Vehicle in subsequent connections.
//
// When the Account was created with [FromTokenSource], the returned vehicle's
// Fleet API connection fetches a current access token for each request.
func (a *Account) GetVehicle(_ context.Context, vin string, privateKey authentication.ECDHPrivateKey, sessions *cache.SessionCache) (*vehicle.Vehicle, error) {
	conn := inet.NewConnection(vin, a.authHeader, a.Host, a.UserAgent)
	if a.tokenSource != nil {
		conn.SetAuthHeaderFunc(a.authorization)
	}
	car, err := vehicle.NewVehicle(conn, privateKey, sessions)
	if err != nil {
		conn.Close()
	}
	return car, err
}

// Get sends an HTTP GET request to endpoint.
//
// The endpoint should contain only the path (e.g., "api/1/vehicles/foo"); the domain is determined
// by the a.Host.
func (a *Account) Get(ctx context.Context, endpoint string) ([]byte, error) {
	url := fmt.Sprintf("https://%s/%s", a.Host, endpoint)
	request, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error constructing request to %s: %w", endpoint, err)
	}
	auth, err := a.authorization()
	if err != nil {
		return nil, err
	}
	log.Debug("Requesting %s...", url)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", a.UserAgent)
	request.Header.Set("Authorization", auth)
	response, err := a.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("error fetching %s: %w", endpoint, err)
	}
	defer func() {
		_ = response.Body.Close()
	}()
	if response.StatusCode != http.StatusOK {
		err := fmt.Errorf("http error when sending command to %s: %s", url, response.Status)
		return nil, err
	}
	reader := io.LimitedReader{R: response.Body, N: connector.MaxResponseLength}
	body, err := io.ReadAll(&reader)
	if err != nil {
		return nil, err
	}
	log.Debug("Received: %s\n", body)
	return body, err
}

func (a *Account) sendFleetAPICommand(ctx context.Context, endpoint string, command interface{}) ([]byte, error) {
	auth, err := a.authorization()
	if err != nil {
		return nil, err
	}
	return inet.SendFleetAPICommand(ctx, &a.client, a.UserAgent, auth, fmt.Sprintf("https://%s/%s", a.Host, endpoint), command)
}

// Post sends an HTTP POST request to endpoint.
//
// The endpoint should contain only the path (e.g., "api/1/vehicles/foo"); the domain is determined
// by the ServerConfig used to create the Account. Returns the HTTP body of the response.
func (a *Account) Post(ctx context.Context, endpoint string, data []byte) ([]byte, error) {
	return a.sendFleetAPICommand(ctx, endpoint, data)
}

// SendVehicleFleetAPICommand sends a command to a vehicle through the REST API.
//
// The command must support JSON serialization.
func (a *Account) SendVehicleFleetAPICommand(ctx context.Context, vin, endpoint string, command interface{}) ([]byte, error) {
	endpoint = fmt.Sprintf("api/1/vehicles/%s/%s", vin, endpoint)
	return a.sendFleetAPICommand(ctx, endpoint, command)
}

// UpdateKey sends metadata about a public key to Tesla's servers.
//
// Vehicles query this information when displaying the list of paired mobile devices and NFC cards
// in the vehicle's Locks screen. Only the account that first registers a public key can modify its
// metadata.
func (a *Account) UpdateKey(ctx context.Context, publicKey *ecdh.PublicKey, name string) error {
	params := map[string]string{
		"public_key": fmt.Sprintf("%02x", publicKey.Bytes()),
		"kind":       "mobile_device",
		"model":      "3rd Party Application",
		"name":       name,
		"tag":        a.UserAgent,
	}
	_, err := a.sendFleetAPICommand(ctx, "api/1/users/keys", &params)
	return err
}
