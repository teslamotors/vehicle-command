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
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/teslamotors/vehicle-command/internal/authentication"
	"github.com/teslamotors/vehicle-command/internal/log"
	"github.com/teslamotors/vehicle-command/pkg/account"
	"github.com/teslamotors/vehicle-command/pkg/cache"
	"github.com/teslamotors/vehicle-command/pkg/connector/inet"
	"github.com/teslamotors/vehicle-command/pkg/protocol"
	"github.com/teslamotors/vehicle-command/pkg/vehicle"
)

const (
	DefaultTimeout       = 10 * time.Second
	maxRequestBodyBytes  = 512
	vinLength            = 17
	proxyProtocolVersion = "tesla-http-proxy/1.1.0"
	MaxResponseLength    = 10000000
	MaxAttempts          = 2
)

var baseDomainRE = regexp.MustCompile(`use base URL: https://([-a-z0-9.]*)`)
var h2Prefix = "h2=https://"

func getAccount(req *http.Request) (*account.Account, error) {
	token, ok := strings.CutPrefix(req.Header.Get("Authorization"), "Bearer ")
	if !ok {
		return nil, fmt.Errorf("client did not provide an OAuth token")
	}
	return account.New(token, proxyProtocolVersion)
}

// Proxy exposes an HTTP API for sending vehicle commands.
type Proxy struct {
	Timeout time.Duration

	commandKey       protocol.ECDHPrivateKey
	sessions         *cache.SessionCache
	vinLock          sync.Map
	unsupported      sync.Map
	domainForSubject sync.Map
}

func (p *Proxy) updateDomainForSubject(subject, domain string) {
	p.domainForSubject.Store(subject, domain)
}

func (p *Proxy) fetchDomainForSubject(subject string) string {
	domain, ok := p.domainForSubject.Load(subject)
	if !ok {
		return ""
	}
	return domain.(string)
}

func (p *Proxy) markUnsupportedVIN(vin string) {
	p.unsupported.Store(vin, true)
}

func (p *Proxy) isNotSupported(vin string) bool {
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
// Vehicles must have the public part of skey enrolled on their keychains. (This is a
// command-authentication key, not a TLS key.)
func New(ctx context.Context, skey protocol.ECDHPrivateKey, cacheSize int) (*Proxy, error) {
	return &Proxy{
		Timeout:    DefaultTimeout,
		commandKey: skey,
		sessions:   cache.New(cacheSize),
	}, nil
}

// Response contains a server's response to a client request.
type Response struct {
	Response   interface{} `json:"response"`
	Error      string      `json:"error"`
	ErrDetails string      `json:"error_description"`
}

type carResponse struct {
	Result bool   `json:"result"`
	Reason string `json:"string"`
}

func writeJSONError(w http.ResponseWriter, code int, err error) {
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
			// Response came from the car as opposed to Tesla's servers
			reply.Response = &carResponse{Reason: err.Error()}
		} else {
			reply.Error = err.Error()
		}
		jsonBytes, err = json.Marshal(&reply)
		if err != nil {
			log.Error("Error serializing reply %+v: %s", &reply, err)
			code = http.StatusInternalServerError
			jsonBytes = []byte("{\"error\": \"internal server error\"}")
		}
	}
	if code != http.StatusOK {
		log.Error("Returning error %s", http.StatusText(code))
	}
	w.WriteHeader(code)
	w.Header().Add("Content-Type", "application/json")
	jsonBytes = append(jsonBytes, '\n')
	w.Write(jsonBytes)
}

var connectionHeaders = []string{
	"Proxy-Connection",
	"Keep-Alive",
	"Transfer-Encoding",
	"Te",
	"Upgrade",
}

// forwardRequest is the fallback handler for "/api/1/*".
// It forwards GET and POST requests to Tesla using the proxy's OAuth token.
func (p *Proxy) forwardRequest(acct *account.Account, w http.ResponseWriter, req *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), p.Timeout)
	defer cancel()

	proxyReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL.String(), req.Body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	proxyReq.Header = req.Header.Clone()
	// Remove per-hop headers
	for _, hdr := range connectionHeaders {
		proxyReq.Header.Del(hdr)
	}

	clientIP, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}

	const xff = "X-Forwarded-For"
	previous := req.Header.Values(xff)
	if len(previous) == 0 {
		proxyReq.Header.Add(xff, clientIP)
	} else {
		previous = append(previous, clientIP)
		// If the client sent multiple XFF headers, flatten them.
		proxyReq.Header.Set(xff, strings.Join(previous, ", "))
	}
	proxyReq.URL.Scheme = "https"
	attempts := 0

	var requestBody []byte
	if req.Body != nil {
		requestBody, err = io.ReadAll(req.Body)
		if err != nil {
			writeJSONError(w, http.StatusBadGateway, err)
			return
		}
		req.Body = io.NopCloser(bytes.NewBuffer(requestBody))
	}

	for {
		proxyReq.URL.Host = acct.Host
		log.Debug("Forwarding request to %s", proxyReq.URL.String())
		client := http.Client{}
		result, err := client.Do(proxyReq)

		if err != nil {
			if urlErr, ok := err.(*url.Error); ok && urlErr.Timeout() {
				writeJSONError(w, http.StatusGatewayTimeout, urlErr)
			} else {
				writeJSONError(w, http.StatusBadGateway, err)
			}
			return
		}

		limitedReader := &io.LimitedReader{R: result.Body, N: MaxResponseLength + 1}
		body, err := io.ReadAll(limitedReader)
		result.Body.Close()

		if err != nil {
			writeJSONError(w, http.StatusBadGateway, err)
			return
		}

		if len(body) == MaxResponseLength+1 {
			writeJSONError(w, http.StatusBadGateway, protocol.NewError("response exceeds maximum length", true, true))
			return
		}

		if result.StatusCode == http.StatusMisdirectedRequest && result.Header.Get("Alt-Svc") != "" {
			altSvc := result.Header.Values("Alt-Svc")
			idx := slices.IndexFunc(altSvc, func(str string) bool { return strings.HasPrefix(str, h2Prefix) })
			if idx == -1 {
				writeJSONError(w, result.StatusCode, err)
				return
			}

			altHost := altSvc[idx][len(h2Prefix):]
			log.Debug("Received HTTP Status 421. Updating server URL to %s", altHost)
			acct.Host = altHost
			p.updateDomainForSubject(acct.Subject, acct.Host)
			if req.Body != nil {
				req.Body = io.NopCloser(bytes.NewBuffer(requestBody))
			}
		} else {
			for _, hdr := range connectionHeaders {
				result.Header.Del(hdr)
			}
			outHeader := w.Header()
			for name, value := range result.Header {
				outHeader[name] = value
			}

			w.WriteHeader(result.StatusCode)
			w.Write(body)
			return
		}

		attempts += 1
		if attempts == MaxAttempts {
			writeJSONError(w, http.StatusBadGateway, protocol.NewError("max retry exhausted", false, false))
		}

		log.Debug("Retrying transmission after error...")
		select {
		case <-ctx.Done():
			writeJSONError(w, http.StatusGatewayTimeout, ctx.Err())
			return
		case <-time.After(1 * time.Second):
			continue
		}
	}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	log.Info("Received %s request for %s", req.Method, req.URL.Path)

	acct, err := getAccount(req)
	if err != nil {
		writeJSONError(w, http.StatusForbidden, err)
		return
	}
	if host := p.fetchDomainForSubject(acct.Subject); host != "" {
		acct.Host = host
	}

	if strings.HasPrefix(req.URL.Path, "/api/1/vehicles/") {
		path := strings.Split(req.URL.Path, "/")
		if len(path) == 7 && path[5] == "command" {
			command := path[6]
			vin := path[4]
			if len(vin) != vinLength {
				writeJSONError(w, http.StatusNotFound, errors.New("expected 17-character VIN in path (do not user Fleet API ID)"))
				return
			}
			if p.isNotSupported(vin) {
				p.forwardRequest(acct, w, req)
				if acct.Host != p.fetchDomainForSubject(acct.Subject) {
					p.updateDomainForSubject(acct.Subject, acct.Host)
				}
			} else {
				if err := p.handleVehicleCommand(acct, w, req, command, vin); err == ErrCommandUseRESTAPI {
					p.forwardRequest(acct, w, req)
				}
			}
			return
		}
		if len(path) == 5 && path[4] == "fleet_telemetry_config" {
			p.handleFleetTelemetryConfig(acct, w, req)
			return
		}
	}
	p.forwardRequest(acct, w, req)
}

func (p *Proxy) handleFleetTelemetryConfig(acct *account.Account, w http.ResponseWriter, req *http.Request) {
	log.Info("Processing fleet telemetry configuration...")
	defer req.Body.Close()
	body, err := io.ReadAll(req.Body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("could not read request body: %s", err))
		return
	}
	var params struct {
		VINs   []string      `json:"vins"`
		Config jwt.MapClaims `json:"config"`
	}
	if err := json.Unmarshal(body, &params); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("could not parse JSON body: %s", err))
		return
	}

	// Let the server validate the VINs and config, the proxy just needs to sign
	if _, ok := params.Config["aud"]; ok {
		log.Warning("Confuration 'aud' field will be overwritten")
	}
	if _, ok := params.Config["iss"]; ok {
		log.Warning("Configuration 'iss' field will be overwritten")
	}
	token, err := authentication.SignMessageForFleet(p.commandKey, "TelemetryClient", params.Config)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Errorf("error signing configuration: %s", err))
		return
	}

	// Forward the new request to Tesla's servers
	jwtRequest := make(map[string]interface{})
	jwtRequest["vins"] = params.VINs
	jwtRequest["token"] = token
	bodyJSON, err := json.Marshal(jwtRequest)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Errorf("error while serializing a request: %s", err))
		return
	}
	req.Body = io.NopCloser(bytes.NewReader(bodyJSON))
	req.URL, err = req.URL.Parse("/api/1/vehicles/fleet_telemetry_config_jws")
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Errorf("error creating proxied URL: %s", err))
		return
	}
	log.Debug("Posting data to %s: %s", req.URL.String(), bodyJSON)
	p.forwardRequest(acct, w, req)
}

func (p *Proxy) handleVehicleCommand(acct *account.Account, w http.ResponseWriter, req *http.Request, command, vin string) error {
	ctx, cancel := context.WithTimeout(context.Background(), p.Timeout)
	defer cancel()

	// Serialize commands sent to a specific VIN to avoid some complexities associated with sharing
	// the vehicle.Vehicle object. VCSEC commands fail if they arrive out of order, anyway.
	if err := p.lockVIN(ctx, vin); err != nil {
		writeJSONError(w, http.StatusServiceUnavailable, err)
		return err
	}
	defer p.unlockVIN(vin)

	car, commandToExecuteFunc, err := p.loadVehicleAndCommandFromRequest(ctx, acct, w, req, command, vin)
	if err != nil {
		return err
	}

	if err := car.Connect(ctx); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return err
	}
	defer car.Disconnect()

	if err := car.StartSession(ctx, nil); errors.Is(err, protocol.ErrProtocolNotSupported) {
		p.markUnsupportedVIN(vin)
		p.forwardRequest(acct, w, req)
		return err
	} else if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return err
	}
	defer car.UpdateCachedSessions(p.sessions)

	if err = commandToExecuteFunc(car); err == ErrCommandUseRESTAPI {
		return err
	}
	if protocol.IsNominalError(err) {
		writeJSONError(w, http.StatusOK, err)
		return err
	}
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return err
	}

	w.Header().Add("Content-Type", "application/json")
	fmt.Fprintln(w, "{\"response\":{\"result\":true,\"reason\":\"\"}}")
	return nil
}

func (p *Proxy) loadVehicleAndCommandFromRequest(ctx context.Context, acct *account.Account, w http.ResponseWriter, req *http.Request,
	command, vin string) (*vehicle.Vehicle, func(*vehicle.Vehicle) error, error) {

	log.Debug("Executing %s on %s", command, vin)
	if req.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, nil)
		return nil, nil, fmt.Errorf("wrong http method")
	}

	commandToExecuteFunc, err := extractCommandAction(ctx, req, command)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return nil, nil, err
	}

	car, err := acct.GetVehicle(ctx, vin, p.commandKey, p.sessions)
	if err != nil || car == nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return nil, nil, err
	}

	return car, commandToExecuteFunc, err
}

func extractCommandAction(ctx context.Context, req *http.Request, command string) (func(*vehicle.Vehicle) error, error) {
	var params RequestParameters
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, &inet.HttpError{Code: http.StatusBadRequest, Message: "could not read request body"}
	}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &params); err != nil {
			return nil, &inet.HttpError{Code: http.StatusBadRequest, Message: "error occurred while parsing request parameters"}
		}
	}

	return ExtractCommandAction(ctx, command, params)
}
