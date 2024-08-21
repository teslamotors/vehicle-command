package main

import (
	"flag"
	"os"
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
	origCert := os.Getenv(EnvTlsCert)
	origKey := os.Getenv(EnvTlsKey)
	origHost := os.Getenv(EnvHost)
	origPort := os.Getenv(EnvPort)
	origVerbose := os.Getenv(EnvVerbose)
	origTimeout := os.Getenv(EnvTimeout)
	origArgs := os.Args
	os.Args = []string{"cmd"}

	defer func() {
		os.Setenv(EnvTlsCert, origCert)
		os.Setenv(EnvTlsKey, origKey)
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
		os.Setenv(EnvTlsCert, "/env/cert.pem")
		os.Setenv(EnvTlsKey, "/env/key.pem")
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
