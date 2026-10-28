package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/teslamotors/vehicle-command/internal/log"
	"github.com/teslamotors/vehicle-command/pkg/cli"
	"github.com/teslamotors/vehicle-command/pkg/protocol"
	"github.com/teslamotors/vehicle-command/pkg/proxy"
)

const (
	cacheSize   = 10000 // Number of cached vehicle sessions
	defaultPort = 443
)

const (
	EnvTLSCert = "TESLA_HTTP_PROXY_TLS_CERT"
	EnvTLSKey  = "TESLA_HTTP_PROXY_TLS_KEY"
	// EnvInsecureHTTP enables plaintext HTTP for deployments where a trusted reverse proxy
	// terminates TLS before forwarding requests to this process.
	EnvInsecureHTTP = "TESLA_HTTP_PROXY_INSECURE_HTTP"
	EnvHost         = "TESLA_HTTP_PROXY_HOST"
	EnvPort         = "TESLA_HTTP_PROXY_PORT"
	EnvTimeout      = "TESLA_HTTP_PROXY_TIMEOUT"
	EnvVerbose      = "TESLA_VERBOSE"
)

const nonLocalhostWarning = `
Do not listen on a network interface without adding client authentication. Unauthorized clients may
be used to create excessive traffic from your IP address to Tesla's servers, which Tesla may respond
to by rate limiting or blocking your connections.`

const sharedKeyWarning = `It is unsafe to use the same private key for TLS and command authentication.

Generate a new TLS key for this server.`

const insecureHTTPWarning = `WARNING: TLS is disabled. The proxy is serving plaintext HTTP.
Only use this mode behind a trusted TLS-terminating reverse proxy, and do not expose this listener directly to an untrusted network.`

var errInsecureHTTPNonLocalhost = errors.New(`-insecure-http is only allowed when the proxy listens on localhost or 127.0.0.1.
Non-localhost plaintext HTTP is intentionally unsupported in tesla-http-proxy; use pkg/proxy to build a custom deployment.`)

// errSharedKey is returned when the TLS key is a copy of the command-authentication key.
var errSharedKey = errors.New(sharedKeyWarning)

type HTTProxyConfig struct {
	keyFilename  string
	certFilename string
	insecureHTTP bool
	verbose      bool
	host         string
	port         int
	timeout      time.Duration
}

var (
	httpConfig = &HTTProxyConfig{}
)

func init() {
	flag.StringVar(&httpConfig.certFilename, "cert", "", "TLS certificate chain `file` with concatenated server, intermediate CA, and root CA certificates")
	flag.StringVar(&httpConfig.keyFilename, "tls-key", "", "Server TLS private key `file`")
	flag.BoolVar(&httpConfig.insecureHTTP, "insecure-http", false, "Serve plaintext HTTP; use only behind a trusted TLS-terminating reverse proxy")
	flag.BoolVar(&httpConfig.verbose, "verbose", false, "Enable verbose logging")
	flag.StringVar(&httpConfig.host, "host", "localhost", "Proxy server `hostname`")
	flag.IntVar(&httpConfig.port, "port", defaultPort, "`Port` to listen on")
	flag.DurationVar(&httpConfig.timeout, "timeout", proxy.DefaultTimeout, "Timeout interval when sending commands")
}

func Usage() {
	out := flag.CommandLine.Output()
	fmt.Fprintf(out, "Usage: %s [OPTION...]\n", os.Args[0])
	fmt.Fprintf(out, "\nA server that exposes a REST API for sending commands to Tesla vehicles")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, nonLocalhostWarning)
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Options:")
	flag.PrintDefaults()
}

// flagWasSet reports whether name was explicitly present on the command line. This matters for
// boolean flags, whose false value otherwise cannot be distinguished from an omitted flag when
// environment configuration is applied.
func flagWasSet(name string) bool {
	wasSet := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			wasSet = true
		}
	})
	return wasSet
}

func isLocalListenHost(host string) bool {
	return host == "localhost" || host == "127.0.0.1"
}


func main() {
	// ******************************************************************************************
	// WHY IS PLAIN HTTP AN EXPLICIT OPT-IN?
	// ******************************************************************************************
	// In the past, we have had problems with third-party applications that made it easy for DIY
	// enthusiasts to inadvertently expose their vehicles to the public Internet. Plain HTTP must
	// therefore never be selected automatically, including when a Cloud Run environment variable
	// is present. The conspicuously named -insecure-http option is only for expert users who have
	// a trusted TLS-terminating reverse proxy in front of this process.
	//
	// Users who cannot make that guarantee can instead use pkg/proxy to build an appropriately
	// secured application around the proxy service.

	config, err := cli.NewConfig(cli.FlagPrivateKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load credential configuration: %s\n", err)
		os.Exit(1)
	}

	flag.Usage = Usage
	config.RegisterCommandLineFlags()
	flag.Parse()
	if err := httpConfig.readFromEnvironment(flagWasSet("insecure-http")); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading environment: %s\n", err)
		os.Exit(1)
	}
	config.ReadFromEnvironment()

	if httpConfig.verbose {
		log.SetLevel(log.LevelDebug)
	}

	if httpConfig.host != "localhost" {
		fmt.Fprintln(os.Stderr, nonLocalhostWarning)
	}
	if httpConfig.insecureHTTP {
		fmt.Fprintln(os.Stderr, insecureHTTPWarning)
		if !isLocalListenHost(httpConfig.host) {
			fmt.Fprintf(os.Stderr, "Error: %s\n", errInsecureHTTPNonLocalhost)
			os.Exit(1)
		}
	}

	// Startup failures are reported here instead of through the logger, which is silent at the
	// default log level.
	if err := run(config, httpConfig); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}

// commandKey loads the private key used to authenticate commands sent to vehicles. When TLS is
// enabled, it also verifies that the key is not serving as the server's TLS key.
func commandKey(config *cli.Config, tlsKeyFilename string, verifyTLSKey bool) (protocol.ECDHPrivateKey, error) {
	skey, err := config.PrivateKey()
	if err != nil {
		return nil, fmt.Errorf("could not load command-authentication private key: %w", err)
	}

	if !verifyTLSKey {
		return skey, nil
	}

	if tlsPublicKey, err := protocol.LoadPublicKey(tlsKeyFilename); err == nil {
		if bytes.Equal(tlsPublicKey.Bytes(), skey.PublicBytes()) {
			return nil, errSharedKey
		}
		log.Debug("Verified that TLS key is not the same as the command-authentication key.")
	} else {
		// Discarding the error here is deliberate
		log.Debug("Verified that TLS key is not a recycled command-authentication key, because it is not NIST P256.")
	}
	return skey, nil
}

// newProxyServer creates the HTTP server that fronts the proxy. TLS is configured when the
// listener is started. Failures are returned instead of terminating the process, so that callers
// report them consistently.
func newProxyServer(config *cli.Config, httpConfig *HTTProxyConfig) (*http.Server, error) {
	skey, err := commandKey(config, httpConfig.keyFilename, !httpConfig.insecureHTTP)
	if err != nil {
		return nil, err
	}

	log.Debug("Creating proxy")
	p, err := proxy.New(context.Background(), skey, cacheSize)
	if err != nil {
		return nil, fmt.Errorf("could not initialize proxy service: %w", err)
	}
	p.Timeout = httpConfig.timeout

	// To add more application logic requests, such as alternative client authentication, create
	// a http.HandleFunc implementation (https://pkg.go.dev/net/http#HandlerFunc). The ServeHTTP
	// method of your implementation can perform your business logic and then, if the request is
	// authorized, invoke p.ServeHTTP. Finally, replace p below with an object of your newly
	// created type.
	return &http.Server{
		Addr:    fmt.Sprintf("%s:%d", httpConfig.host, httpConfig.port),
		Handler: p,
	}, nil
}

// serve accepts connections on listener until server stops. By default connections use TLS; plain
// HTTP is available only with the explicit insecureHTTP option. A deliberate shutdown is not an
// error.
func serve(server *http.Server, listener net.Listener, certFilename, keyFilename string, insecureHTTP bool) error {
	var err error
	if insecureHTTP {
		err = server.Serve(listener)
	} else {
		err = server.ServeTLS(listener, certFilename, keyFilename)
	}
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// run starts the proxy and blocks until it stops serving, returning the reason it stopped.
func run(config *cli.Config, httpConfig *HTTProxyConfig) error {
	server, err := newProxyServer(config, httpConfig)
	if err != nil {
		return err
	}

	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return err
	}
	// Serving can return before closing the listener, for example if TLS credentials cannot load.
	defer func() { _ = listener.Close() }()

	log.Info("Listening on %s", listener.Addr())
	return serve(server, listener, httpConfig.certFilename, httpConfig.keyFilename, httpConfig.insecureHTTP)
}

// readFromEnvironment applies configuration from environment variables. Values are not
// overwritten. insecureHTTPFlagSet distinguishes an explicit false flag from an omitted one.
func (httpConfig *HTTProxyConfig) readFromEnvironment(insecureHTTPFlagSet bool) error {
	if httpConfig.certFilename == "" {
		httpConfig.certFilename = os.Getenv(EnvTLSCert)
	}

	if httpConfig.keyFilename == "" {
		httpConfig.keyFilename = os.Getenv(EnvTLSKey)
	}

	if !insecureHTTPFlagSet && !httpConfig.insecureHTTP {
		if insecureHTTP, ok := os.LookupEnv(EnvInsecureHTTP); ok {
			value, err := strconv.ParseBool(insecureHTTP)
			if err != nil {
				return fmt.Errorf("invalid insecure HTTP value: %s", insecureHTTP)
			}
			httpConfig.insecureHTTP = value
		}
	}

	if httpConfig.host == "localhost" {
		host, ok := os.LookupEnv(EnvHost)
		if ok {
			httpConfig.host = host
		}
	}

	if !httpConfig.verbose {
		if verbose, ok := os.LookupEnv(EnvVerbose); ok {
			httpConfig.verbose = verbose != "false" && verbose != "0"
		}
	}

	var err error
	if httpConfig.port == defaultPort {
		if port, ok := os.LookupEnv(EnvPort); ok {
			httpConfig.port, err = strconv.Atoi(port)
			if err != nil {
				return fmt.Errorf("invalid port: %s", port)
			}
		}
	}

	if httpConfig.timeout == proxy.DefaultTimeout {
		if timeoutEnv, ok := os.LookupEnv(EnvTimeout); ok {
			httpConfig.timeout, err = time.ParseDuration(timeoutEnv)
			if err != nil {
				return fmt.Errorf("invalid timeout: %s", timeoutEnv)
			}
		}
	}

	return nil
}
