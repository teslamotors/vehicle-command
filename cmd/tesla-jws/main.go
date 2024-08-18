package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/golang-jwt/jwt/v5"

	"github.com/teslamotors/vehicle-command/internal/authentication"
	"github.com/teslamotors/vehicle-command/pkg/cli"
)

const helpStr = `
usage: tesla-jws [OPTION...] sign APP [JSON_FILE]
			Generates a JWS (JSON Web Signature) for JSON_FILE.
       tesla-jws verify [JWS_FILE]
			Verifies that the signature on JWS_FILE is correct, but does not check that the issuer
			is trusted or that the audience is correct.

Creates or verifies a JWS (JSON Web Siganture) using Schnorr/P256 signatures. This signature type is
not part of the JWS standard, but permits clients to safely re-use existing ECDH/P256 keys as
signing keys.

The JSON_FILE may contain standard JWT (JSON Web Token) claims, such as an expiration time.
However, the audience ("aud") and issuer ("iss") will, if present, be overwritten.

This implementation parses the issuer as a base64-encoded public key and uses it to verify the JWS.
The client must verify that the issuer is trusted.`

func readStdinOrFile(filenamePosition int) ([]byte, error) {
	if flag.NArg() <= filenamePosition {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(flag.Arg(filenamePosition))
}

func readClaims(argNumber int) (jwt.MapClaims, error) {
	jsonBytes, err := readStdinOrFile(argNumber)
	if err != nil {
		return nil, err
	}
	var claims jwt.MapClaims
	if err := json.Unmarshal(jsonBytes, &claims); err != nil {
		return nil, err
	}
	return claims, nil
}

func usage() {
	fmt.Println(helpStr)
	fmt.Println("")
	flag.PrintDefaults()
}

func sign(config *cli.Config, fleet bool) {
	skey, err := config.PrivateKey()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load private key: %s\n", err)
		os.Exit(1)
	}
	if skey == nil {
		fmt.Fprintln(os.Stderr, "No private key specified.")
		os.Exit(1)
	}
	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Missing APP context string.")
		os.Exit(1)
	}
	application := flag.Arg(1)
	claims, err := readClaims(2)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading JSON: %s\n", err)
		os.Exit(1)
	}
	var token string
	if fleet {
		token, err = authentication.SignMessageForFleet(skey, application, claims)
	} else {
		if config.VIN == "" {
			fmt.Fprintln(os.Stderr, "Provide either -vin or -fleet")
			os.Exit(1)
		}
		token, err = authentication.SignMessageForVehicle(skey, config.VIN, application, claims)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create JWS: %s\n", err)
		os.Exit(1)
	}
	fmt.Println(token)
}

// Warning: You probably don't want to re-use this function. It does not verify that the issuer is
// trusted. See note about using 'verify' in command-line usage.
func extractIssuerPublicKey(token *jwt.Token) (interface{}, error) {
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		// Shouldn't be reachable, since jwt.MapClaims is the default
		return nil, fmt.Errorf("could not parse JWT claims")
	}
	// Since we're treating the issuer as an encoded public key ([]byte) that has the same type
	// as other algorithms, we need to verify the algorithm type explicitly.
	if alg, ok := token.Header["alg"]; !ok {
		return nil, fmt.Errorf("JWT is missing signature algorithm in header")
	} else if algStr, ok := alg.(string); !ok || algStr != authentication.TeslaSchnorrSHA256 {
		return nil, fmt.Errorf("unsupported signature type")
	}
	issuer, ok := claims["iss"]
	if !ok {
		return nil, fmt.Errorf("JWT is missing issuer")
	}
	issuerB64, ok := issuer.(string)
	if !ok {
		return nil, fmt.Errorf("issuer field is not a string")
	}
	publicKeyBytes, err := base64.StdEncoding.DecodeString(issuerB64)
	if err != nil {
		return nil, fmt.Errorf("issuer is not a base64-encoded string")
	}
	return publicKeyBytes, nil
}

func verify() {
	tokenBytes, err := readStdinOrFile(1)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read token: %s\n", err)
		os.Exit(1)
	}
	token, err := jwt.ParseWithClaims(string(tokenBytes), jwt.MapClaims{}, extractIssuerPublicKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid JWT: %s\n", err)
		os.Exit(1)
	}
	claims, err := json.Marshal(token.Claims)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to encode claims as JSON: %s\n", err)
		os.Exit(1)
	}
	fmt.Printf("%s\n", claims)
}

func main() {
	var fleet bool
	flag.Usage = usage
	flag.BoolVar(&fleet, "fleet", false, "Sign fleet-wide message")

	config, err := cli.NewConfig(cli.FlagPrivateKey | cli.FlagVIN)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create configuration: %s\n", err)
		os.Exit(1)
	}

	config.RegisterCommandLineFlags()
	flag.Parse()
	config.ReadFromEnvironment()

	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "Missing command (verify/sign)")
		os.Exit(1)
	}

	switch flag.Arg(0) {
	case "sign":
		sign(config, fleet)
	case "verify":
		verify()
	default:
		fmt.Fprintln(os.Stderr, "Unrecognized command")
		os.Exit(1)
	}
}
