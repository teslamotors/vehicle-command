// Utility for generating, saving, and migrating keys

package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/teslamotors/vehicle-command/internal/authentication"
	"github.com/teslamotors/vehicle-command/internal/log"
	"github.com/teslamotors/vehicle-command/pkg/cli"
	"github.com/teslamotors/vehicle-command/pkg/protocol"
)

func writeErr(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format, a...)
	fmt.Fprintf(os.Stderr, "\n")
}

const usageText = `
Creates or deletes a private key and saves it in the system keyring, or migrates a key from a
plaintext file into the system keyring.

The program writes the public key to stdout (except when deleting a key). When using the create
option, the program will not overwrite an existing unless invoked with -f.

The type of keyring and name of the key inside that keyring are controlled by the command-line
options below, or through the corresponding environment variables.`

func cliUsage() {
	usage(flag.CommandLine.Output())
}

func usage(w io.Writer) {
	fmt.Fprintf(w, "usage: %s [OPTION...] create|delete|export|migrate\n", filepath.Base(os.Args[0]))
	fmt.Fprintln(w, usageText)
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "OPTIONS:")
	flag.PrintDefaults()
}

func printPublicKey(skey protocol.ECDHPrivateKey) bool {
	pkey := ecdsa.PublicKey{Curve: elliptic.P256()}
	pkey.X, pkey.Y = elliptic.Unmarshal(elliptic.P256(), skey.PublicBytes())
	if pkey.X == nil {
		return false
	}
	derPublicKey, err := x509.MarshalPKIXPublicKey(&pkey)
	if err != nil {
		return false
	}
	pem.Encode(os.Stdout, &pem.Block{Type: "PUBLIC KEY", Bytes: derPublicKey})
	return true
}

func printPrivateKey(skey protocol.ECDHPrivateKey) error {
	native, ok := skey.(*authentication.NativeECDHKey)
	if !ok {
		return fmt.Errorf("private key is not exportable")
	}
	derPrivateKey, err := x509.MarshalECPrivateKey(native.PrivateKey)
	if err != nil {
		return err
	}
	pem.Encode(os.Stdout, &pem.Block{Type: "EC PRIVATE KEY", Bytes: derPrivateKey})
	return nil
}

func main() {
	// Command-line variables
	var (
		overwrite bool
		skey      protocol.ECDHPrivateKey
		err       error
	)
	status := 1
	defer func() {
		os.Exit(status)
	}()

	config, err := cli.NewConfig(cli.FlagOAuth | cli.FlagPrivateKey)
	config.RegisterCommandLineFlags()
	flag.Usage = cliUsage
	flag.BoolVar(&overwrite, "f", false, "Overwrite existing key if it exists")
	flag.Parse()
	if config.Debug {
		log.SetLevel(log.LevelDebug)
	}
	if err != nil {
		writeErr("Failed to load credential configuration: %s", err)
		return
	}
	config.ReadFromEnvironment()

	if flag.NArg() != 1 {
		usage(os.Stderr)
		return
	}

	switch flag.Arg(0) {
	case "migrate":
		if config.KeyFilename == "" || config.KeyringKeyName == "" {
			writeErr("Must provide path of existing key (-key-file) and name of new key (-key-name)")
			return
		}

		skey, err = protocol.LoadPrivateKey(config.KeyFilename)
		if err != nil {
			writeErr("Unable to read key: %s", err)
			return
		}
		config.KeyFilename = "" // Prevent key from being re-written to a file
	case "delete":
		if err := config.DeletePrivateKey(); err != nil {
			writeErr("Failed to delete key: %s", err)
		} else {
			status = 0
		}
		return
	case "create":
		if !overwrite {
			// Print key and exit if it already exists
			skey, err = config.PrivateKey()
			if err == nil {
				if ok := printPublicKey(skey); !ok {
					writeErr("Failed to parse key. The keyring may be corrupted. Run with -f to generate new key.")
					return
				}
				status = 0
				return
			}
		}
		skey, err = authentication.NewECDHPrivateKey(rand.Reader)
		if err != nil {
			writeErr("Failed to generate private key: %s", err)
			return
		}
	case "export":
		skey, err = config.PrivateKey()
		if err == nil {
			err = printPrivateKey(skey)
		}
		if err != nil {
			writeErr("Failed to export private key: %s", err)
		}
		return
	default:
		writeErr("Unrecognized command-line argument.")
		writeErr("")
		usage(os.Stderr)
		return
	}

	if err = config.SavePrivateKey(skey); err != nil {
		writeErr("Failed to save key to keyring: %s", err)
		return
	}

	if ok := printPublicKey(skey); !ok {
		writeErr("Failed to extract public key. Run with -f to generate new key pair.")
		return
	}
	status = 0
}
