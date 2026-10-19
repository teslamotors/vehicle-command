package main

import (
	"bytes"
	"context"
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
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/teslamotors/vehicle-command/pkg/proxy"
)

// assertEquals should be replaced with a real assertion library
func assertEquals(t *testing.T, expected, actual interface{}, message string) {
	t.Helper()
	if expected != actual {
		t.Errorf("%s: expected %v, got %v", message, expected, actual)
	}
}

func TestParseConfig(t *testing.T) {
	origCert := os.Getenv(EnvTLSCert)
	origKey := os.Getenv(EnvTLSKey)
	origHost := os.Getenv(EnvHost)
	origPort := os.Getenv(EnvPort)
	origVerbose := os.Getenv(EnvVerbose)
	origTimeout := os.Getenv(EnvTimeout)
	origArgs := os.Args
	os.Args = []string{"cmd"}

	defer func() {
		os.Setenv(EnvTLSCert, origCert)
		os.Setenv(EnvTLSKey, origKey)
		os.Setenv(EnvHost, origHost)
		os.Setenv(EnvPort, origPort)
		os.Setenv(EnvVerbose, origVerbose)
		os.Setenv(EnvTimeout, origTimeout)
		os.Args = origArgs
	}()

	t.Run("default values", func(t *testing.T) {
		err := readFromEnvironment()
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		assertEquals(t, "localhost", httpConfig.host, "host")
		assertEquals(t, defaultPort, httpConfig.port, "port")
		assertEquals(t, proxy.DefaultTimeout, httpConfig.timeout, "timeout")
		assertEquals(t, "", httpConfig.certFilename, "certFilename")
		assertEquals(t, "", httpConfig.keyFilename, "keyFilename")
		assertEquals(t, false, httpConfig.verbose, "verbose")
	})

	t.Run("environment variables", func(t *testing.T) {
		os.Setenv(EnvTLSCert, "/env/cert.pem")
		os.Setenv(EnvTLSKey, "/env/key.pem")
		os.Setenv(EnvHost, "envhost")
		os.Setenv(EnvPort, "8443")
		os.Setenv(EnvVerbose, "true")
		os.Setenv(EnvTimeout, "30s")

		err := readFromEnvironment()
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		assertEquals(t, "/env/cert.pem", httpConfig.certFilename, "certFilename")
		assertEquals(t, "/env/key.pem", httpConfig.keyFilename, "keyFilename")
		assertEquals(t, "envhost", httpConfig.host, "host")
		assertEquals(t, 8443, httpConfig.port, "port")
		assertEquals(t, 30*time.Second, httpConfig.timeout, "timeout")
		assertEquals(t, true, httpConfig.verbose, "verbose")
	})

	t.Run("flags override environment variables", func(t *testing.T) {
		os.Args = []string{"cmd", "-cert", "/flag/cert.pem", "-tls-key", "/flag/key.pem", "-host", "flaghost", "-port", "9090", "-timeout", "60s"}

		flag.Parse()
		err := readFromEnvironment()
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		assertEquals(t, "/flag/cert.pem", httpConfig.certFilename, "certFilename")
		assertEquals(t, "/flag/key.pem", httpConfig.keyFilename, "keyFilename")
		assertEquals(t, "flaghost", httpConfig.host, "host")
		assertEquals(t, 9090, httpConfig.port, "port")
		assertEquals(t, 60*time.Second, httpConfig.timeout, "timeout")
	})
}

// proxyBinary is the command built by TestMain. The startup tests below run it as a separate
// process because they assert on its exit status, which is not observable in-process. Building and
// running it is skipped under -short, which leaves the in-process tests in this package.
var proxyBinary string

func TestMain(m *testing.M) {
	// testing.Short is only readable once the test flags are parsed, which m.Run would do.
	flag.Parse()
	var buildDir string
	if !testing.Short() {
		dir, err := os.MkdirTemp("", "tesla-http-proxy-test")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create build directory: %s\n", err)
			os.Exit(1)
		}
		buildDir = dir
		proxyBinary = filepath.Join(buildDir, "tesla-http-proxy")
		if out, err := exec.Command(goCommand(), "build", "-o", proxyBinary, ".").CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to build tesla-http-proxy: %s\n%s", err, out)
			_ = os.RemoveAll(buildDir)
			os.Exit(1)
		}
	}
	code := m.Run()
	if buildDir != "" {
		if err := os.RemoveAll(buildDir); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to remove build directory: %s\n", err)
		}
	}
	os.Exit(code)
}

// goCommand returns the go command of the toolchain running the tests, falling back to a PATH
// lookup.
func goCommand() string {
	binary := filepath.Join(runtime.GOROOT(), "bin", "go")
	if _, err := os.Stat(binary); err != nil {
		return "go"
	}
	return binary
}

// requireProxyBinary skips a process-level test when -short left the command unbuilt.
func requireProxyBinary(t *testing.T) {
	t.Helper()
	if proxyBinary == "" {
		t.Skip("Skipping process-level startup test because -short does not build the command")
	}
}

// credentials are disposable key files generated for a single test.
type credentials struct {
	authKey string // Command-authentication private key (-key-file)
	cert    string // TLS certificate chain (-cert)
	tlsKey  string // TLS private key (-tls-key)
	certPEM []byte
}

// args returns the command line for the proxy. An empty authKey omits -key-file. Tests that never
// connect to the proxy pass port 0, so the OS assigns an unused port and the proxy cannot fail with
// a bind error that hides the failure under test.
func (c *credentials) args(port int) []string {
	args := []string{"-cert", c.cert, "-tls-key", c.tlsKey, "-port", strconv.Itoa(port)}
	if c.authKey != "" {
		args = append(args, "-key-file", c.authKey)
	}
	return args
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

// occupiedPort returns a loopback port that stays bound for the duration of the test. Both
// loopback addresses are bound because "localhost" may resolve to either one.
func occupiedPort(t *testing.T) int {
	t.Helper()
	listener := listenLoopback(t)
	t.Cleanup(func() {
		_ = listener.Close()
	})
	port := listener.Addr().(*net.TCPAddr).Port
	if listener6, err := net.Listen("tcp", fmt.Sprintf("[::1]:%d", port)); err == nil {
		t.Cleanup(func() {
			_ = listener6.Close()
		})
	}
	return port
}

// freePort returns a loopback port that is available when the function returns. Only the tests
// that connect to the proxy need this; failure cases pass port 0 and let the OS choose.
func freePort(t *testing.T) int {
	t.Helper()
	listener := listenLoopback(t)
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatalf("Failed to release loopback port: %s", err)
	}
	return port
}

// proxyEnv isolates the proxy from the developer's home directory, session cache, and TESLA_*
// settings, including TESLA_VERBOSE.
func proxyEnv(t *testing.T) []string {
	t.Helper()
	return []string{"HOME=" + t.TempDir(), "PATH=" + os.Getenv("PATH")}
}

// runProxy runs the proxy to completion and returns its exit status and output.
func runProxy(t *testing.T, args ...string) (exitCode int, stdout, stderr string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, proxyBinary, args...)
	cmd.Env = proxyEnv(t)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("Proxy did not exit; stderr: %s", errBuf.String())
	}
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		t.Fatalf("Failed to run proxy: %s", err)
	}
	return cmd.ProcessState.ExitCode(), outBuf.String(), errBuf.String()
}

// TestStartupFailuresReportErrors covers https://github.com/teslamotors/vehicle-command/issues/521:
// startup failures must exit non-zero and explain themselves on stderr without -verbose.
func TestStartupFailuresReportErrors(t *testing.T) {
	requireProxyBinary(t)
	for _, test := range []struct {
		name string
		// setup optionally invalidates c and returns the command line to run.
		setup func(t *testing.T, c *credentials) []string
		// want is a substring that must appear on stderr.
		want string
	}{
		{
			name: "missing TLS certificate",
			setup: func(_ *testing.T, c *credentials) []string {
				c.cert = filepath.Join(filepath.Dir(c.cert), "missing-cert.pem")
				return c.args(0)
			},
			want: "missing-cert.pem",
		},
		{
			name: "missing TLS key",
			setup: func(_ *testing.T, c *credentials) []string {
				c.tlsKey = filepath.Join(filepath.Dir(c.tlsKey), "missing-key.pem")
				return c.args(0)
			},
			want: "missing-key.pem",
		},
		{
			name: "unreadable TLS certificate",
			setup: func(t *testing.T, c *credentials) []string {
				makeUnreadable(t, c.cert)
				return c.args(0)
			},
			want: "permission denied",
		},
		{
			name: "unreadable TLS key",
			setup: func(t *testing.T, c *credentials) []string {
				makeUnreadable(t, c.tlsKey)
				return c.args(0)
			},
			want: "permission denied",
		},
		{
			// The key stays valid, so the certificate is what the proxy rejects. Only the
			// error report is asserted: the parse error comes from crypto/tls and names
			// neither the file nor a stable reason.
			name: "malformed TLS certificate",
			setup: func(t *testing.T, c *credentials) []string {
				if err := os.WriteFile(c.cert, []byte("this is not a certificate\n"), 0600); err != nil {
					t.Fatalf("Failed to write certificate: %s", err)
				}
				return c.args(0)
			},
			want: "Error:",
		},
		{
			name: "port already in use",
			setup: func(t *testing.T, c *credentials) []string {
				return c.args(occupiedPort(t))
			},
			want: "address already in use",
		},
		{
			name: "port already in use with verbose logging",
			setup: func(t *testing.T, c *credentials) []string {
				return append(c.args(occupiedPort(t)), "-verbose")
			},
			want: "address already in use",
		},
		{
			// Already handled correctly before the fix; asserted to prevent a regression.
			name: "missing command-authentication key",
			setup: func(_ *testing.T, c *credentials) []string {
				c.authKey = ""
				return c.args(0)
			},
			want: "private key",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := newCredentials(t, elliptic.P256())
			exitCode, stdout, stderr := runProxy(t, test.setup(t, &c)...)
			assertEquals(t, 1, exitCode, "exit status")
			assertEquals(t, "", stdout, "stdout")
			if !strings.Contains(stderr, test.want) {
				t.Errorf("stderr does not contain %q: %s", test.want, stderr)
			}
		})
	}
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

// TestStartupRejectsRecycledAuthenticationKey checks that reusing the command-authentication key
// as the TLS key is still refused, now with a non-zero exit status and without echoing the key.
func TestStartupRejectsRecycledAuthenticationKey(t *testing.T) {
	requireProxyBinary(t)
	c := newCredentials(t, elliptic.P256())
	keyPEM, err := os.ReadFile(c.authKey)
	if err != nil {
		t.Fatalf("Failed to read key: %s", err)
	}
	c.tlsKey = c.authKey

	exitCode, stdout, stderr := runProxy(t, c.args(0)...)
	assertEquals(t, 1, exitCode, "exit status")
	assertEquals(t, "", stdout, "stdout")
	if !strings.Contains(stderr, "unsafe to use the same private key") {
		t.Errorf("stderr does not explain the rejection: %s", stderr)
	}
	// The key body is the second line of the PEM file.
	keyBody := strings.Split(strings.TrimSpace(string(keyPEM)), "\n")[1]
	if strings.Contains(stdout+stderr, keyBody) {
		t.Error("Output contains private key material")
	}
}

// TestStartupServesTLS checks that a valid configuration still serves a verified HTTPS request and
// reports no error while doing so. The proxy installs no signal handler, so the test terminates it
// by cancelling its context rather than exercising a shutdown path.
func TestStartupServesTLS(t *testing.T) {
	requireProxyBinary(t)
	for _, test := range []struct {
		name  string
		curve elliptic.Curve
	}{
		{name: "P-256", curve: elliptic.P256()},
		{name: "P-384", curve: elliptic.P384()},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := newCredentials(t, test.curve)
			port := freePort(t)

			ctx, cancel := context.WithCancel(context.Background())
			cmd := exec.CommandContext(ctx, proxyBinary, c.args(port)...)
			cmd.Env = proxyEnv(t)
			var outBuf, errBuf bytes.Buffer
			cmd.Stdout = &outBuf
			cmd.Stderr = &errBuf
			if err := cmd.Start(); err != nil {
				t.Fatalf("Failed to start proxy: %s", err)
			}
			// Output buffers are only safe to read after Wait returns.
			var once sync.Once
			wait := func() {
				once.Do(func() {
					_ = cmd.Wait()
				})
			}
			t.Cleanup(func() {
				cancel()
				wait()
			})

			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(c.certPEM) {
				t.Fatal("Failed to load test certificate")
			}
			client := &http.Client{
				Timeout: time.Second,
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12},
				},
			}
			healthURL := fmt.Sprintf("https://localhost:%d/health", port)

			var status int
			var lastErr error
			deadline := time.Now().Add(30 * time.Second)
			for time.Now().Before(deadline) {
				response, err := client.Get(healthURL)
				if err != nil {
					lastErr = err
					time.Sleep(50 * time.Millisecond)
					continue
				}
				status = response.StatusCode
				_ = response.Body.Close()
				break
			}
			if status == 0 {
				t.Fatalf("Proxy never served %s: %s", healthURL, lastErr)
			}
			assertEquals(t, http.StatusOK, status, "status code")

			cancel()
			wait()
			assertEquals(t, "", outBuf.String(), "stdout")
			if strings.Contains(errBuf.String(), "Error") {
				t.Errorf("Proxy reported an error during normal operation: %s", errBuf.String())
			}
		})
	}
}
