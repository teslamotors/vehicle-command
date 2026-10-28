package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/teslamotors/vehicle-command/pkg/cli"
	"github.com/teslamotors/vehicle-command/pkg/proxy"
)

// assertEquals should be replaced with a real assertion library
func assertEquals(t *testing.T, expected, actual interface{}, message string) {
	t.Helper()
	if expected != actual {
		t.Errorf("%s: expected %v, got %v", message, expected, actual)
	}
}

// unsetEnvironment unsets names for the duration of t and restores both their values and whether
// they were originally set. This keeps configuration tests independent of the caller's shell.
func unsetEnvironment(t *testing.T, names ...string) {
	t.Helper()
	type originalValue struct {
		value string
		set   bool
	}
	original := make(map[string]originalValue, len(names))
	for _, name := range names {
		value, set := os.LookupEnv(name)
		original[name] = originalValue{value: value, set: set}
		if err := os.Unsetenv(name); err != nil {
			t.Fatalf("Failed to unset %s: %s", name, err)
		}
	}
	t.Cleanup(func() {
		for _, name := range names {
			value := original[name]
			var err error
			if value.set {
				err = os.Setenv(name, value.value)
			} else {
				err = os.Unsetenv(name)
			}
			if err != nil {
				t.Errorf("Failed to restore %s: %s", name, err)
			}
		}
	})
}

func TestParseConfig(t *testing.T) {
	origArgs := os.Args
	origConfig := *httpConfig
	os.Args = []string{"cmd"}
	t.Cleanup(func() {
		os.Args = origArgs
		*httpConfig = origConfig
	})
	unsetEnvironment(t,
		EnvTLSCert,
		EnvTLSKey,
		EnvInsecureHTTP,
		EnvHost,
		EnvPort,
		EnvVerbose,
		EnvTimeout,
	)
	*httpConfig = HTTProxyConfig{
		host:    "localhost",
		port:    defaultPort,
		timeout: proxy.DefaultTimeout,
	}

	t.Run("default values", func(t *testing.T) {
		err := httpConfig.readFromEnvironment(false)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		assertEquals(t, "localhost", httpConfig.host, "host")
		assertEquals(t, defaultPort, httpConfig.port, "port")
		assertEquals(t, proxy.DefaultTimeout, httpConfig.timeout, "timeout")
		assertEquals(t, "", httpConfig.certFilename, "certFilename")
		assertEquals(t, "", httpConfig.keyFilename, "keyFilename")
		assertEquals(t, false, httpConfig.insecureHTTP, "insecureHTTP")
		assertEquals(t, false, httpConfig.verbose, "verbose")
	})

	t.Run("environment variables", func(t *testing.T) {
		os.Setenv(EnvTLSCert, "/env/cert.pem")
		os.Setenv(EnvTLSKey, "/env/key.pem")
		os.Setenv(EnvInsecureHTTP, "true")
		os.Setenv(EnvHost, "envhost")
		os.Setenv(EnvPort, "8443")
		os.Setenv(EnvVerbose, "true")
		os.Setenv(EnvTimeout, "30s")

		err := httpConfig.readFromEnvironment(false)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		assertEquals(t, "/env/cert.pem", httpConfig.certFilename, "certFilename")
		assertEquals(t, "/env/key.pem", httpConfig.keyFilename, "keyFilename")
		assertEquals(t, true, httpConfig.insecureHTTP, "insecureHTTP")
		assertEquals(t, "envhost", httpConfig.host, "host")
		assertEquals(t, 8443, httpConfig.port, "port")
		assertEquals(t, 30*time.Second, httpConfig.timeout, "timeout")
		assertEquals(t, true, httpConfig.verbose, "verbose")
	})

	t.Run("flags override environment variables", func(t *testing.T) {
		os.Setenv(EnvInsecureHTTP, "true")
		os.Args = []string{"cmd", "-cert", "/flag/cert.pem", "-tls-key", "/flag/key.pem", "-insecure-http=false", "-host", "flaghost", "-port", "9090", "-timeout", "60s"}

		flag.Parse()
		if !flagWasSet("insecure-http") {
			t.Fatal("insecure-http flag was not recorded as set")
		}
		err := httpConfig.readFromEnvironment(flagWasSet("insecure-http"))
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		assertEquals(t, "/flag/cert.pem", httpConfig.certFilename, "certFilename")
		assertEquals(t, "/flag/key.pem", httpConfig.keyFilename, "keyFilename")
		assertEquals(t, false, httpConfig.insecureHTTP, "insecureHTTP")
		assertEquals(t, "flaghost", httpConfig.host, "host")
		assertEquals(t, 9090, httpConfig.port, "port")
		assertEquals(t, 60*time.Second, httpConfig.timeout, "timeout")
	})
}

func TestReadFromEnvironmentInsecureHTTP(t *testing.T) {
	newHTTPConfig := func() *HTTProxyConfig {
		return &HTTProxyConfig{
			certFilename: "cert.pem",
			keyFilename:  "key.pem",
			verbose:      true,
			host:         "example.test",
			port:         1,
			timeout:      time.Hour,
		}
	}

	t.Run("enabled", func(t *testing.T) {
		t.Setenv(EnvInsecureHTTP, "true")
		httpConfig := newHTTPConfig()
		if err := httpConfig.readFromEnvironment(false); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		assertEquals(t, true, httpConfig.insecureHTTP, "insecureHTTP")
	})

	t.Run("does not override flag", func(t *testing.T) {
		t.Setenv(EnvInsecureHTTP, "false")
		httpConfig := newHTTPConfig()
		httpConfig.insecureHTTP = true
		if err := httpConfig.readFromEnvironment(true); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		assertEquals(t, true, httpConfig.insecureHTTP, "insecureHTTP")
	})

	t.Run("explicitly disabled flag overrides environment", func(t *testing.T) {
		t.Setenv(EnvInsecureHTTP, "true")
		httpConfig := newHTTPConfig()
		if err := httpConfig.readFromEnvironment(true); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		assertEquals(t, false, httpConfig.insecureHTTP, "insecureHTTP")
	})

	t.Run("invalid value", func(t *testing.T) {
		t.Setenv(EnvInsecureHTTP, "not-a-bool")
		err := newHTTPConfig().readFromEnvironment(false)
		if err == nil || !strings.Contains(err.Error(), "invalid insecure HTTP value") {
			t.Fatalf("Expected invalid insecure HTTP error, got %v", err)
		}
	})
}

// credentials are disposable key files generated for a single test.
type credentials struct {
	authKey string // Command-authentication private key
	cert    string // TLS certificate chain
	tlsKey  string // TLS private key
	certPEM []byte
}

// newCredentials generates a command-authentication key plus an independent self-signed TLS
// certificate and key on curve, all in a per-test temporary directory.
func newCredentials(t *testing.T, curve elliptic.Curve) credentials {
	t.Helper()
	dir := t.TempDir()
	c := credentials{
		authKey: filepath.Join(dir, "auth-key.pem"),
		cert:    filepath.Join(dir, "tls-cert.pem"),
		tlsKey:  filepath.Join(dir, "tls-key.pem"),
	}
	// The command-authentication key must be NIST P256.
	writePrivateKey(t, c.authKey, generateKey(t, elliptic.P256()))

	tlsKey := generateKey(t, curve)
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IsCA:         true,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &tlsKey.PublicKey, tlsKey)
	if err != nil {
		t.Fatalf("Failed to create certificate: %s", err)
	}
	c.certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	if err := os.WriteFile(c.cert, c.certPEM, 0600); err != nil {
		t.Fatalf("Failed to write certificate: %s", err)
	}
	writePrivateKey(t, c.tlsKey, tlsKey)
	return c
}

// config returns credential settings that load the generated command-authentication key. The
// session cache is disabled and no keyring name is set, so the developer's cache file and system
// keyring are never consulted.
func (c *credentials) config(t *testing.T) *cli.Config {
	t.Helper()
	config, err := cli.NewConfig(cli.FlagPrivateKey)
	if err != nil {
		t.Fatalf("Failed to create credential configuration: %s", err)
	}
	config.KeyFilename = c.authKey
	config.DisableCache = true
	return config
}

// proxyConfig returns proxy settings for a server on an ephemeral loopback port. Tests build these
// directly rather than registering and parsing the global command-line flags.
func (c *credentials) proxyConfig() *HTTProxyConfig {
	return &HTTProxyConfig{
		certFilename: c.cert,
		keyFilename:  c.tlsKey,
		host:         "127.0.0.1",
		port:         0,
		timeout:      proxy.DefaultTimeout,
	}
}

func generateKey(t *testing.T, curve elliptic.Curve) *ecdsa.PrivateKey {
	t.Helper()
	skey, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %s", err)
	}
	return skey
}

func writePrivateKey(t *testing.T, filename string, skey *ecdsa.PrivateKey) {
	t.Helper()
	der, err := x509.MarshalECPrivateKey(skey)
	if err != nil {
		t.Fatalf("Failed to marshal key: %s", err)
	}
	if err := os.WriteFile(filename, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}), 0600); err != nil {
		t.Fatalf("Failed to write key: %s", err)
	}
}

// listenLoopback binds an ephemeral loopback port and returns the listener.
func listenLoopback(t *testing.T) net.Listener {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to bind loopback port: %s", err)
	}
	return listener
}

// occupiedPort returns a loopback port that stays bound for the duration of the test.
func occupiedPort(t *testing.T) int {
	t.Helper()
	listener := listenLoopback(t)
	t.Cleanup(func() {
		_ = listener.Close()
	})
	return listener.Addr().(*net.TCPAddr).Port
}

// makeUnreadable removes all permissions from filename.
func makeUnreadable(t *testing.T, filename string) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("Skipping permission test because root can read files with mode 000")
	}
	if err := os.Chmod(filename, 0); err != nil {
		t.Fatalf("Failed to change permissions: %s", err)
	}
}

// TestRunReportsStartupFailures covers https://github.com/teslamotors/vehicle-command/issues/521:
// a proxy that cannot start must report the reason rather than return nothing for main to discard.
// Each case runs on an ephemeral port unless it is testing the bind itself, so that an unrelated
// bind error cannot stand in for the failure under test.
func TestRunReportsStartupFailures(t *testing.T) {
	for _, test := range []struct {
		name string
		// setup invalidates the configuration in the way the case is named after, and returns a
		// substring that the resulting error must contain.
		setup func(t *testing.T, config *cli.Config, httpConfig *HTTProxyConfig) string
		// check makes any additional assertion about the kind of error returned.
		check func(t *testing.T, err error, httpConfig *HTTProxyConfig)
	}{
		{
			name: "missing TLS certificate",
			setup: func(_ *testing.T, _ *cli.Config, httpConfig *HTTProxyConfig) string {
				httpConfig.certFilename = filepath.Join(filepath.Dir(httpConfig.certFilename), "missing-cert.pem")
				return "missing-cert.pem"
			},
		},
		{
			name: "missing TLS key",
			setup: func(_ *testing.T, _ *cli.Config, httpConfig *HTTProxyConfig) string {
				httpConfig.keyFilename = filepath.Join(filepath.Dir(httpConfig.keyFilename), "missing-key.pem")
				return "missing-key.pem"
			},
		},
		{
			name: "unreadable TLS certificate",
			setup: func(t *testing.T, _ *cli.Config, httpConfig *HTTProxyConfig) string {
				makeUnreadable(t, httpConfig.certFilename)
				return httpConfig.certFilename
			},
			check: func(t *testing.T, err error, httpConfig *HTTProxyConfig) {
				assertPermissionError(t, err, httpConfig.certFilename)
			},
		},
		{
			name: "unreadable TLS key",
			setup: func(t *testing.T, _ *cli.Config, httpConfig *HTTProxyConfig) string {
				makeUnreadable(t, httpConfig.keyFilename)
				return httpConfig.keyFilename
			},
			check: func(t *testing.T, err error, httpConfig *HTTProxyConfig) {
				assertPermissionError(t, err, httpConfig.keyFilename)
			},
		},
		{
			// Only the certificate is replaced, so the valid key cannot satisfy the assertion.
			// crypto/tls reports which input it could not parse but not its pathname, so the
			// distinct reason is asserted instead of a filename.
			name: "malformed TLS certificate",
			setup: func(t *testing.T, _ *cli.Config, httpConfig *HTTProxyConfig) string {
				writeGarbage(t, httpConfig.certFilename)
				return "PEM data in certificate input"
			},
		},
		{
			name: "malformed TLS key",
			setup: func(t *testing.T, _ *cli.Config, httpConfig *HTTProxyConfig) string {
				writeGarbage(t, httpConfig.keyFilename)
				return "PEM data in key input"
			},
		},
		{
			// Already handled correctly before the fix; asserted to prevent a regression.
			name: "missing command-authentication key",
			setup: func(_ *testing.T, config *cli.Config, _ *HTTProxyConfig) string {
				config.KeyFilename = ""
				return "command-authentication private key"
			},
		},
		{
			name: "port already in use",
			setup: func(t *testing.T, _ *cli.Config, httpConfig *HTTProxyConfig) string {
				httpConfig.port = occupiedPort(t)
				return fmt.Sprintf("%s:%d", httpConfig.host, httpConfig.port)
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := newCredentials(t, elliptic.P256())
			config := c.config(t)
			httpConfig := c.proxyConfig()
			want := test.setup(t, config, httpConfig)

			err := run(config, httpConfig)
			if err == nil {
				t.Fatal("run returned no error")
			}
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error does not contain %q: %s", want, err)
			}
			if test.check != nil {
				test.check(t, err, httpConfig)
			}
		})
	}
}

// assertPermissionError checks that err identifies filename as the file it could not read, which
// tells an unreadable TLS key apart from an unreadable certificate.
func assertPermissionError(t *testing.T, err error, filename string) {
	t.Helper()
	if !errors.Is(err, os.ErrPermission) {
		t.Errorf("error is not a permission error: %s", err)
	}
	var pathError *os.PathError
	if !errors.As(err, &pathError) {
		t.Fatalf("error does not identify a file: %s", err)
	}
	assertEquals(t, filename, pathError.Path, "unreadable file")
}

func writeGarbage(t *testing.T, filename string) {
	t.Helper()
	if err := os.WriteFile(filename, []byte("this is not PEM data\n"), 0600); err != nil {
		t.Fatalf("Failed to write %s: %s", filename, err)
	}
}

// TestNewProxyServerRejectsRecycledCommandKey checks that a TLS key which is a copy of the
// command-authentication key is still refused, with instructions and without echoing the key. The
// copy has its own pathname, so the rejection can only come from comparing public keys.
func TestNewProxyServerRejectsRecycledCommandKey(t *testing.T) {
	c := newCredentials(t, elliptic.P256())
	keyPEM, err := os.ReadFile(c.authKey)
	if err != nil {
		t.Fatalf("Failed to read key: %s", err)
	}
	recycled := filepath.Join(filepath.Dir(c.authKey), "recycled-tls-key.pem")
	if err := os.WriteFile(recycled, keyPEM, 0600); err != nil {
		t.Fatalf("Failed to write key: %s", err)
	}

	httpConfig := c.proxyConfig()
	httpConfig.keyFilename = recycled
	_, err = newProxyServer(c.config(t), httpConfig)
	if !errors.Is(err, errSharedKey) {
		t.Fatalf("Expected a shared key error, got %v", err)
	}
	for _, want := range []string{
		"It is unsafe to use the same private key for TLS and command authentication.",
		"Generate a new TLS key for this server.",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not contain %q: %s", want, err)
		}
	}
	// The key body is the second line of the PEM file.
	keyBody := strings.Split(strings.TrimSpace(string(keyPEM)), "\n")[1]
	if strings.Contains(err.Error(), keyBody) {
		t.Error("error contains private key material")
	}
}

// TestNewProxyServerPlainHTTPDoesNotInspectTLSKey verifies that plaintext mode leaves TLS
// credentials entirely unused. In particular, it must not treat the command key as a TLS key.
func TestNewProxyServerPlainHTTPDoesNotInspectTLSKey(t *testing.T) {
	c := newCredentials(t, elliptic.P256())
	httpConfig := c.proxyConfig()
	httpConfig.insecureHTTP = true
	httpConfig.keyFilename = c.authKey

	server, err := newProxyServer(c.config(t), httpConfig)
	if err != nil {
		t.Fatalf("Failed to create plaintext server: %s", err)
	}
	if err := server.Close(); err != nil {
		t.Errorf("Failed to close server: %s", err)
	}
}

// TestServeTLS checks that a valid configuration serves a verified HTTPS request and reports no
// error when the server is shut down.
func TestServeTLS(t *testing.T) {
	for _, test := range []struct {
		name  string
		curve elliptic.Curve
	}{
		{name: "P-256", curve: elliptic.P256()},
		{name: "P-384", curve: elliptic.P384()},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := newCredentials(t, test.curve)
			server, err := newProxyServer(c.config(t), c.proxyConfig())
			if err != nil {
				t.Fatalf("Failed to create server: %s", err)
			}
			// The listener is bound before the request is sent, so the test does not need to
			// poll for the server to come up or reserve a port in advance.
			listener := listenLoopback(t)
			t.Cleanup(func() {
				_ = server.Close()
				_ = listener.Close()
			})
			served := make(chan error, 1)
			go func() {
				served <- serve(server, listener, c.cert, c.tlsKey, false)
			}()

			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(c.certPEM) {
				t.Fatal("Failed to load test certificate")
			}
			client := &http.Client{
				Timeout: 10 * time.Second,
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12},
				},
			}
			response, err := client.Get(fmt.Sprintf("https://%s/health", listener.Addr()))
			if err != nil {
				t.Fatalf("Failed to query the proxy: %s", err)
			}
			assertEquals(t, http.StatusOK, response.StatusCode, "status code")
			if err := response.Body.Close(); err != nil {
				t.Errorf("Failed to close response body: %s", err)
			}

			if err := server.Close(); err != nil {
				t.Errorf("Failed to close server: %s", err)
			}
			if err := <-served; err != nil {
				t.Errorf("Server stopped with an error: %s", err)
			}
		})
	}
}

// TestServePlainHTTPWithoutTLS checks that an explicit insecureHTTP configuration can start and
// serve a request without a certificate or TLS key. The default-mode startup-failure tests above
// continue to cover the TLS-required behavior when this opt-in is absent.
func TestServePlainHTTPWithoutTLS(t *testing.T) {
	c := newCredentials(t, elliptic.P256())
	httpConfig := c.proxyConfig()
	httpConfig.certFilename = ""
	httpConfig.keyFilename = ""
	httpConfig.insecureHTTP = true
	server, err := newProxyServer(c.config(t), httpConfig)
	if err != nil {
		t.Fatalf("Failed to create plaintext server: %s", err)
	}
	listener := listenLoopback(t)
	t.Cleanup(func() {
		_ = server.Close()
		_ = listener.Close()
	})
	served := make(chan error, 1)
	go func() {
		served <- serve(server, listener, httpConfig.certFilename, httpConfig.keyFilename, httpConfig.insecureHTTP)
	}()

	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Get(fmt.Sprintf("http://%s/health", listener.Addr()))
	if err != nil {
		t.Fatalf("Failed to query plaintext proxy: %s", err)
	}
	assertEquals(t, http.StatusOK, response.StatusCode, "status code")
	if err := response.Body.Close(); err != nil {
		t.Errorf("Failed to close response body: %s", err)
	}

	if err := server.Close(); err != nil {
		t.Errorf("Failed to close server: %s", err)
	}
	if err := <-served; err != nil {
		t.Errorf("Server stopped with an error: %s", err)
	}
}

func TestIsLocalListenHost(t *testing.T) {
	if !isLocalListenHost("localhost") || !isLocalListenHost("127.0.0.1") {
		t.Fatal("localhost and 127.0.0.1 must be allowed")
	}
	if isLocalListenHost("0.0.0.0") || isLocalListenHost("example.com") {
		t.Fatal("non-local hosts must be rejected for insecure HTTP")
	}
}
