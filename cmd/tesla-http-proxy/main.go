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
	EnvHost    = "TESLA_HTTP_PROXY_HOST"
	EnvPort    = "TESLA_HTTP_PROXY_PORT"
	EnvTimeout = "TESLA_HTTP_PROXY_TIMEOUT"
	EnvVerbose = "TESLA_VERBOSE"
)

const nonLocalhostWarning = `
Do not listen on a network interface without adding client authentication. Unauthorized clients may
be used to create excessive traffic from your IP address to Tesla's servers, which Tesla may respond
to by rate limiting or blocking your connections.`

const sharedKeyWarning = `It is unsafe to use the same private key for TLS and command authentication.

Generate a new TLS key for this server.`

// errSharedKey is returned when the TLS key is a copy of the command-authentication key.
var errSharedKey = errors.New(sharedKeyWarning)

type HTTProxyConfig struct {
	keyFilename  string
	certFilename string
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

func main() {
	// ******************************************************************************************
	// WHY IS THERE NO OPTION FOR DISABLING TLS?
	// ******************************************************************************************
	// In the past, we have had problems with third-party applications that made it easy for DIY
	// enthusiasts to inadvertently expose their vehicles to the public Internet. In order to
	// protect users who do not understand the risks of disabling TLS, we decided to omit an
	// --insecure flag or similar.
	//
	// Expert users who need to disable TLS can do so without forking this repository by using the
	// pkg/proxy package, which is agnostic to TLS. This application is a very thin wrapper around
	// that package.

	config, err := cli.NewConfig(cli.FlagPrivateKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load credential configuration: %s\n", err)
		os.Exit(1)
	}

	flag.Usage = Usage
	config.RegisterCommandLineFlags()
	flag.Parse()
	if err := readFromEnvironment(); err != nil {
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

	// Startup failures are reported here instead of through the logger, which is silent at the
	// default log level.
	if err := run(config, httpConfig); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}

// commandKey loads the private key used to authenticate commands sent to vehicles, and verifies
// that it is not also serving as the server's TLS key.
func commandKey(config *cli.Config, tlsKeyFilename string) (protocol.ECDHPrivateKey, error) {
	skey, err := config.PrivateKey()
	if err != nil {
		return nil, fmt.Errorf("could not load command-authentication private key: %w", err)
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

// newProxyServer creates the HTTPS server that fronts the proxy. Failures are returned instead of
// terminating the process, so that callers report them consistently.
func newProxyServer(config *cli.Config, httpConfig *HTTProxyConfig) (*http.Server, error) {
	skey, err := commandKey(config, httpConfig.keyFilename)
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

// serve accepts TLS connections on listener until server stops. A deliberate shutdown is not an
// error.
func serve(server *http.Server, listener net.Listener, certFilename, keyFilename string) error {
	if err := server.ServeTLS(listener, certFilename, keyFilename); !errors.Is(err, http.ErrServerClosed) {
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
	// ServeTLS leaves the listener open if it cannot load the certificate or key.
	defer func() { _ = listener.Close() }()

	log.Info("Listening on %s", listener.Addr())
	return serve(server, listener, httpConfig.certFilename, httpConfig.keyFilename)
}

// readConfig applies configuration from environment variables.
// Values are not overwritten.
func readFromEnvironment() error {
	if httpConfig.certFilename == "" {
		httpConfig.certFilename = os.Getenv(EnvTLSCert)
	}

	if httpConfig.keyFilename == "" {
		httpConfig.keyFilename = os.Getenv(EnvTLSKey)
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
