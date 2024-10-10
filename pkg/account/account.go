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
	UserAgent  string
	authHeader string
	Host       string
	Subject    string
	client     http.Client
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
// Optional userAgent can be passed in - otherwise it will be generated from code
func New(oauthToken, userAgent string) (*Account, error) {
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

// GetVehicle returns the Vehicle belonging to the account with the provided vin.
//
// Providing a nil privateKey is allowed, but a privateKey is required for most Vehicle
// interactions. Typically, the privateKey will only be nil when connecting to the Vehicle to send
// an AddKeyRequest; see documentation in [pkg/github.com/teslamotors/vehicle-command/pkg/vehicle]. The
// sessions parameter may also be nil, but providing a cache.SessionCache avoids a round-trip
// handshake with the Vehicle in subsequent connections.
func (a *Account) GetVehicle(ctx context.Context, vin string, privateKey authentication.ECDHPrivateKey, sessions *cache.SessionCache) (*vehicle.Vehicle, error) {
	conn := inet.NewConnection(vin, a.authHeader, a.Host, a.UserAgent)
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
	log.Debug("Requesting %s...", url)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", a.UserAgent)
	request.Header.Set("Authorization", a.authHeader)
	response, err := a.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("error fetching %s: %w", endpoint, err)
	}
	defer response.Body.Close()
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
	return inet.SendFleetAPICommand(ctx, &a.client, a.UserAgent, a.authHeader, fmt.Sprintf("https://%s/%s", a.Host, endpoint), command)
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
