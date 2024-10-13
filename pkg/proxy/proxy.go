package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/teslamotors/vehicle-command/internal/authentication"
	"github.com/teslamotors/vehicle-command/internal/log"
	"github.com/teslamotors/vehicle-command/pkg/cache"
	"github.com/teslamotors/vehicle-command/pkg/connector/inet"
	"github.com/teslamotors/vehicle-command/pkg/protocol"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/universalmessage"
)

const (
	DefaultTimeout       = 10 * time.Second
	vinLength            = 17
	proxyProtocolVersion = "tesla-http-proxy/1.1.0"
	xForwardedForHeader  = "X-Forwarded-For"
)

type Vehicle interface {
	Connect(context.Context) error
	Disconnect()
	StartSession(context.Context, []universalmessage.Domain) error
	UpdateCachedSessions(*cache.SessionCache) error
	ExecuteAction(context.Context, interface{}) error
}

type Account interface {
	GetVehicle(context.Context, string, authentication.ECDHPrivateKey, *cache.SessionCache) (Vehicle, error)
	GetHost() string
}

type AccountProvider func(oauthToken, userAgent string) (Account, error)

// Response contains a server's response to a client request.
type Response struct {
	Response   interface{} `json:"response"`
	Error      string      `json:"error"`
	ErrDetails string      `json:"error_description"`
}

// vehicleResponse is the response format used by the vehicle's command API.
type vehicleResponse struct {
	Result bool   `json:"result"`
	Reason string `json:"string"`
}

// connectionHeaders are HTTP headers that are not forwarded.
var connectionHeaders = []string{
	"Proxy-Connection",
	"Keep-Alive",
	"Transfer-Encoding",
	"Te",
	"Upgrade",
}

// contextKey is a type used to store values in context.Context.
type contextKey string

const accountContext contextKey = "account"

// Proxy exposes an HTTP API for sending vehicle commands.
type Proxy struct {
	Timeout time.Duration

	commandKey      protocol.ECDHPrivateKey
	sessions        *cache.SessionCache
	vinLock         sync.Map
	unsupported     sync.Map
	handler         http.Handler
	accountProvider AccountProvider
}

func (p *Proxy) markSignedCommandsUnsupportedVIN(vin string) {
	p.unsupported.Store(vin, true)
}

func (p *Proxy) signedCommandUnsupported(vin string) bool {
	_, ok := p.unsupported.Load(vin)
	return ok
}

// lockVIN locks a VIN-specific mutex, blocking until the operation succeeds or ctx expires.
func (p *Proxy) lockVIN(ctx context.Context, vin string) error {
	lock := make(chan bool, 1)
	for {
		if obj, loaded := p.vinLock.LoadOrStore(vin, lock); loaded {
			select {
			case <-obj.(chan bool):
				// The goroutine that reads from the channel doesn't necessarily own the mutex. This
				// allows the mutex owner to delete the entry from the map, limiting the size of the
				// map to the number of concurrent vehicle commands.
			case <-ctx.Done():
				return ctx.Err()
			}
		} else {
			return nil
		}
	}
}

// unlockVIN releases a VIN-specific mutex.
func (p *Proxy) unlockVIN(vin string) {
	obj, ok := p.vinLock.Load(vin)
	if !ok {
		panic("called unlock without owning mutex")
	}
	p.vinLock.Delete(vin)  // Allow someone else to claim the mutex
	close(obj.(chan bool)) // Unblock goroutines
}

// New creates an http proxy.
//
// Vehicles must have the public part of skey enrolled on their keychains.
// (This is a command-authentication key, not a TLS key.)
func New(ctx context.Context, skey protocol.ECDHPrivateKey, cacheSize int, accountProvider AccountProvider) *Proxy {
	proxy := &Proxy{
		Timeout:         DefaultTimeout,
		commandKey:      skey,
		sessions:        cache.New(cacheSize),
		accountProvider: accountProvider,
	}
	proxy.setupHandlers()
	return proxy
}

// setupHandlers sets up the HTTP handlers for the proxy.
func (p *Proxy) setupHandlers() {
	handler := http.NewServeMux()
	handler.HandleFunc("POST /api/1/vehicles/fleet_telemetry_config", p.handleFleetTelemetryConfig)
	handler.HandleFunc("POST /api/1/vehicles/{vin}/command/{command}", p.handleVehicleCommand)
	handler.HandleFunc("/api/1/", p.forwardRequest)
	p.handler = handler
}

// ServeHTTP validates authentication, sets context, and passes the request to the appropriate handler.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	ctx, cancel := context.WithTimeout(req.Context(), p.Timeout)
	defer cancel()

	log.Info("Received %s request for %s", req.Method, req.URL.Path)

	acct, err := p.getAccount(req)
	if err != nil {
		writeResponseError(w, http.StatusUnauthorized, err)
		return
	}

	p.handler.ServeHTTP(w, req.WithContext(context.WithValue(ctx, accountContext, acct)))
}

// forwardRequest is the fallback handler for "/api/1/*".
// It forwards requests to Tesla using the proxy's OAuth token.
func (p *Proxy) forwardRequest(w http.ResponseWriter, req *http.Request) {
	proxyReq, err := http.NewRequestWithContext(req.Context(), req.Method, req.URL.String(), req.Body)
	if err != nil {
		writeResponseError(w, http.StatusBadRequest, err)
		return
	}
	proxyReq.Header = req.Header.Clone()
	// Remove per-hop headers
	for _, hdr := range connectionHeaders {
		proxyReq.Header.Del(hdr)
	}

	clientIP, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		writeResponseError(w, http.StatusBadRequest, err)
		return
	}
	proxyReq.Header.Set(xForwardedForHeader, strings.Join(append(req.Header.Values(xForwardedForHeader), clientIP), ", "))

	acct := req.Context().Value(accountContext).(Account)
	proxyReq.URL.Host = acct.GetHost()
	proxyReq.URL.Scheme = "https"

	log.Debug("Forwarding request to %s", proxyReq.URL.String())
	client := http.Client{}
	resp, err := client.Do(proxyReq)
	if err != nil {
		if urlErr, ok := err.(*url.Error); ok && urlErr.Timeout() {
			writeResponseError(w, http.StatusGatewayTimeout, urlErr)
		} else {
			writeResponseError(w, http.StatusBadGateway, err)
		}
		return
	}
	defer resp.Body.Close()

	for _, hdr := range connectionHeaders {
		resp.Header.Del(hdr)
	}
	outHeader := w.Header()
	for name, value := range resp.Header {
		outHeader[name] = value
	}

	w.WriteHeader(resp.StatusCode)
	w.Header().Add("Content-Type", resp.Header.Get("Content-Type"))
	io.Copy(w, resp.Body)
}

type fleetTelemetryConfig struct {
	VINs   []string      `json:"vins"`
	Config jwt.MapClaims `json:"config"`
}

func (p *Proxy) handleFleetTelemetryConfig(w http.ResponseWriter, req *http.Request) {
	log.Info("Processing fleet telemetry configuration...")
	defer req.Body.Close()
	body, err := io.ReadAll(req.Body)
	if err != nil {
		writeResponseError(w, http.StatusBadRequest, fmt.Errorf("could not read request body: %s", err))
		return
	}

	var params fleetTelemetryConfig
	if err := json.Unmarshal(body, &params); err != nil {
		writeResponseError(w, http.StatusBadRequest, fmt.Errorf("could not parse JSON body: %s", err))
		return
	}

	// Let the server validate the VINs and config, the proxy just needs to sign
	if _, ok := params.Config["aud"]; ok {
		log.Warning("Configuration 'aud' field will be overwritten")
	}
	if _, ok := params.Config["iss"]; ok {
		log.Warning("Configuration 'iss' field will be overwritten")
	}
	token, err := authentication.SignMessageForFleet(p.commandKey, "TelemetryClient", params.Config)
	if err != nil {
		writeResponseError(w, http.StatusInternalServerError, fmt.Errorf("error signing configuration: %s", err))
		return
	}

	// Forward the new request to Tesla's servers
	jwtRequest := map[string]interface{}{
		"vins":  params.VINs,
		"token": token,
	}
	bodyJSON, err := json.Marshal(jwtRequest)
	if err != nil {
		writeResponseError(w, http.StatusInternalServerError, fmt.Errorf("error while serializing a request: %s", err))
		return
	}

	req.Body = io.NopCloser(bytes.NewReader(bodyJSON))
	req.URL, err = req.URL.Parse("/api/1/vehicles/fleet_telemetry_config_jws")
	if err != nil {
		writeResponseError(w, http.StatusInternalServerError, fmt.Errorf("error creating proxied URL: %s", err))
		return
	}

	log.Debug("Posting data to %s: %s", req.URL.String(), bodyJSON)
	p.forwardRequest(w, req)
}

func (p *Proxy) handleVehicleCommand(w http.ResponseWriter, req *http.Request) {
	vin := req.PathValue("vin")
	if len(vin) != vinLength {
		writeResponseError(w, http.StatusNotFound, errors.New("expected 17-character VIN in path (do not use vehicle ID)"))
		return
	}
	command := req.PathValue("command")
	acct, ok := req.Context().Value(accountContext).(Account)
	if !ok {
		writeResponseError(w, http.StatusInternalServerError, errors.New("internal server error"))
		return
	}

	if p.signedCommandUnsupported(vin) {
		p.forwardRequest(w, req)
		return
	}

	params, err := p.parseRequestParameters(req)
	if err != nil {
		writeResponseError(w, http.StatusBadRequest, err)
		return
	}

	action, err := ExtractCommandAction(command, params)
	if err == nil && action == nil {
		writeResponseError(w, http.StatusNotFound, errors.New("unknown command"))
		return
	}
	if err != nil {
		writeResponseError(w, http.StatusBadRequest, err)
		return
	}

	ctx := req.Context()
	if err := p.lockVIN(ctx, vin); err != nil {
		writeResponseError(w, http.StatusServiceUnavailable, err)
		return
	}
	defer p.unlockVIN(vin)

	vehicle, err := acct.GetVehicle(ctx, vin, p.commandKey, p.sessions)
	if err != nil || vehicle == nil {
		writeResponseError(w, http.StatusInternalServerError, err)
		return
	}

	if err := p.sendActionToVehicle(ctx, vehicle, action); err != nil {
		if errors.Is(err, protocol.ErrProtocolNotSupported) {
			p.markSignedCommandsUnsupportedVIN(vin)
			p.forwardRequest(w, req)
			return
		}
		writeResponseError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, []byte(`{"response":{"result":true,"reason":""}}`))
}

func (p *Proxy) parseRequestParameters(req *http.Request) (RequestParameters, error) {
	var params RequestParameters
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return params, fmt.Errorf("could not read request body: %s", err)
	}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &params); err != nil {
			return params, fmt.Errorf("error occurred while parsing request parameters: %s", err)
		}
	}
	return params, nil
}

// sendActionToVehicle connects to the vehicle, starts a session, and executes the action.
func (p *Proxy) sendActionToVehicle(ctx context.Context, vehicle Vehicle, action interface{}) error {
	if err := vehicle.Connect(ctx); err != nil {
		return err
	}
	defer vehicle.Disconnect()

	if err := vehicle.StartSession(ctx, nil); err != nil {
		return err
	}

	defer vehicle.UpdateCachedSessions(p.sessions)

	return vehicle.ExecuteAction(ctx, action)
}

func (p *Proxy) getAccount(req *http.Request) (Account, error) {
	token, ok := strings.CutPrefix(req.Header.Get("Authorization"), "Bearer ")
	if !ok {
		return nil, fmt.Errorf("client did not provide an OAuth token")
	}
	return p.accountProvider(token, proxyProtocolVersion)
}

func writeResponseError(w http.ResponseWriter, code int, err error) {
	reply := Response{}

	var httpErr *inet.HttpError
	var jsonBytes []byte
	if errors.As(err, &httpErr) {
		code = httpErr.Code
		jsonBytes = []byte(err.Error())
	} else {
		if err == nil {
			reply.Error = http.StatusText(code)
		} else if protocol.IsNominalError(err) {
			// Response came from the vehicle as opposed to Tesla's servers
			reply.Response = &vehicleResponse{Reason: err.Error()}
		} else {
			reply.Error = err.Error()
		}
		jsonBytes, err = json.Marshal(&reply)
		if err != nil {
			log.Error("Error serializing reply %+v: %s", &reply, err)
			code = http.StatusInternalServerError
			jsonBytes = []byte(`{"error": "internal server error"}`)
		}
	}
	if code != http.StatusOK {
		log.Error("Returning error %s", http.StatusText(code))
	}
	writeJSON(w, code, jsonBytes)
}

func writeJSON(w http.ResponseWriter, code int, jsonBytes []byte) {
	w.WriteHeader(code)
	w.Header().Add("Content-Type", "application/json")
	w.Write(jsonBytes)
}
