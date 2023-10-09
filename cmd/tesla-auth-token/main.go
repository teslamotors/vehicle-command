// Utility for fetching OAuth tokens

package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/teslamotors/vehicle-command/pkg/cli"
)

func usage() {
	w := flag.CommandLine.Output()
	fmt.Fprintf(w, "usage: %s [-token-name token_name] [file]\n", filepath.Base(os.Args[0]))
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Reads OAuth token from stdin or file and saves it under token_name in the system")
	fmt.Fprintf(w, "keyring. The token_name defaults to $%s.\n", cli.EnvTeslaTokenName)
}

func main() {
	returnCode := 1
	defer func() {
		os.Exit(returnCode)
	}()

	config, err := cli.NewConfig(cli.FlagOAuth)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load credential configuration: %s\n", err)
		return
	}

	flag.StringVar(&config.KeyringTokenName, "token-name", "", "Name to use for keyring entry")
	flag.Usage = usage
	flag.Parse()
	config.ReadFromEnvironment()

	if config.KeyringTokenName == "" {
		fmt.Fprintf(os.Stderr, "Must provide system keyring name to save OAuth token under using -token-name or $%s\n", cli.EnvTeslaTokenName)
		return
	}

	var token []byte
	switch flag.NArg() {
	case 0:
		token, err = io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading token from stdin: %s\n", err)
			return
		}
	case 1:
		token, err = os.ReadFile(flag.Arg(0))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading token from file: %s\n", err)
			return
		}
	default:
		fmt.Fprintln(os.Stderr, "Too many command-line arguments")
		return
	}

	if err := config.SaveTokenToKeyring(string(token)); err != nil {
		fmt.Fprintf(os.Stderr, "Error saving token to keyring: %s", err)
		return
	}

	returnCode = 0
}
