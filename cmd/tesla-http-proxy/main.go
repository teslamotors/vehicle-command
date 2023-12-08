package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/teslamotors/vehicle-command/internal/log"
	"github.com/teslamotors/vehicle-command/pkg/cli"
	"github.com/teslamotors/vehicle-command/pkg/protocol"
	"github.com/teslamotors/vehicle-command/pkg/proxy"
)

const (
	cacheSize   = 10000 // Number of cached vehicle sessions
	defaultPort = 443
)

const warning = `
Do not listen on a network interface without adding client authentication. Unauthorized clients may
be used to create excessive traffic from your IP address to Tesla's servers, which Tesla may respond
to by rate limiting or blocking your connections.`

func Usage() {
	out := flag.CommandLine.Output()
	fmt.Fprintf(out, "Usage: %s [OPTION...]\n", os.Args[0])
	fmt.Fprintf(out, "\nA server that exposes a REST API for sending commands to Tesla vehicles")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, warning)
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Options:")
	flag.PrintDefaults()
}

func main() {
	// Command-line options
	var (
		keyFilename  string
		certFilename string
		verbose      bool
		host         string
		port         int
	)

	config, err := cli.NewConfig(cli.FlagPrivateKey)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load credential configuration: %s\n", err)
		os.Exit(1)
	}

	defer func() {
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}
	}()

	flag.StringVar(&certFilename, "cert", "", "TLS certificate chain `file` with concatenated server, intermediate CA, and root CA certificates")
	flag.StringVar(&keyFilename, "tls-key", "", "Server TLS private key `file`")
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose logging")
	flag.StringVar(&host, "host", "localhost", "Proxy server `hostname`")
	flag.IntVar(&port, "port", defaultPort, "`Port` to listen on")
	flag.Usage = Usage
	config.RegisterCommandLineFlags()
	flag.Parse()
	config.ReadFromEnvironment()

	if verbose {
		log.SetLevel(log.LevelDebug)
	}

	if host != "localhost" {
		fmt.Fprintln(os.Stderr, warning)
	}

	var skey protocol.ECDHPrivateKey
	skey, err = config.PrivateKey()
	if err != nil {
		return
	}

	if tlsPublicKey, err := protocol.LoadPublicKey(keyFilename); err == nil {
		if bytes.Equal(tlsPublicKey.Bytes(), skey.PublicBytes()) {
			fmt.Fprintln(os.Stderr, "It is unsafe to use the same private key for TLS and command authentication.")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "Generate a new TLS key for this server.")
			return
		}
	}

	log.Debug("Creating proxy")
	p, err := proxy.New(context.Background(), skey, cacheSize)
	if err != nil {
		return
	}
	addr := fmt.Sprintf("%s:%d", host, port)
	log.Info("Listening on %s", addr)

	// To add more application logic requests, such as alternative client authentication, create
	// a http.HandleFunc implementation (https://pkg.go.dev/net/http#HandlerFunc). The ServeHTTP
	// method of your implementation can perform your business logic and then, if the request is
	// authorized, invoke p.ServeHTTP. Finally, replace p in the below ListenAndServeTLS call with
	// an object of your newly created type.
	log.Error("Server stopped: %s", http.ListenAndServeTLS(addr, certFilename, keyFilename, p))
}
