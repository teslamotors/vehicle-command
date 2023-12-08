# Tesla Vehicle Command SDK
[![Go Reference](https://pkg.go.dev/badge/github.com/teslamotors/vehicle-command/pkg.svg)](https://pkg.go.dev/github.com/teslamotors/vehicle-command/pkg)
[![Build and Test](https://github.com/teslamotors/vehicle-command/actions/workflows/build.yml/badge.svg?branch=main)](https://github.com/teslamotors/vehicle-command/actions/workflows/build.yml)

Tesla vehicles now support a protocol that provides end-to-end command
authentication. This Golang package uses the new protocol to control vehicle
functions, such as climate control and charging.

Among the included tools is an HTTP proxy server that converts REST API calls
to the new vehicle-command protocol.

Some developers may be familiar with Tesla's Owner API. Owner API will stop
working as vehicles begin requiring end-to-end command authentication. If you
are one of these developers, you can set up the proxy server or refactor your
application to use this library directly. Pre-2021 Model S and X vehicles do
not support this new protocol. [Fleet
API](https://developer.tesla.com/docs/tesla-fleet-api#fleet-api) will continue
to work on these vehicles.

## System overview

Command authentication takes place in two steps:

 1. Tesla's servers will only forward messages to a vehicle if the client has a
    valid [OAuth token](https://oauth.net/2/).
 2. The vehicle will only execute the command if it can be authenticated using a
    public key from the vehicle's keychain.

So in order to send a command to a vehicle, a third-party application must
obtain a valid OAuth token from the user, and the user must enroll the
application's public key in the vehicle.

Tesla's website has [instructions for obtaining OAuth
tokens](https://developer.tesla.com/docs/fleet-api). This README has
instructions for generating private keys and directing the user to the
public-key enrollment flow. The tools in this repository can use the OAuth
token and the private key to send commands to vehicles.

For example, the repository includes a [command-line interface](cmd/tesla-control/README.md):

```bash
tesla-control -ble -key-file private_key.pem lock
```

And a REST API proxy server (which is provided with a private key on launch and
uses OAuth tokens sent by clients):

```
curl --cacert cert.pem \
    --header 'Content-Type: application/json' \
    --header "Authorization: Bearer $TESLA_AUTH_TOKEN" \
    --data '{}' \
    "https://localhost:4443/api/1/vehicles/$VIN/command/door_lock"
```

## Installation and configuration

Requirements:

 * You've [installed Golang](https://go.dev/doc/install). The package was
   tested with Go 1.20.
 * You're using macOS or Linux. (Everything except BLE should run on Windows,
   but Windows is not officially supported).

Installation steps:

 1. Download dependencies: `go get ./...`
 1. Compile tools and examples: `go build ./...`
 1. Install tools to your PATH: `go install ./...`

The final command installs the following utilities:

 * **tesla-keygen**: Generate a command-authentication private key and
   save it to your system keyring.
 * **tesla-control**: Send commands to a vehicle over BLE or the Internet. See
   [tool's README file](cmd/tesla-control/README.md) for more information.
 * **tesla-http-proxy**: An HTTP proxy that exposes a REST API for sending
   vehicle commands.
 * **tesla-auth-token**: Write an OAuth token to your system keyring. This
   utility does not fetch tokens. Read the [Fleet API documentation](https://developer.tesla.com/docs/fleet-api#authentication)
   for information on fetching OAuth tokens.

Configure environment variables (optional):

For convenience, you can define the following environment variables to be used
in lieu of command-line flags when using the above applications:

 * `TESLA_KEY_NAME` used to derive the entry name for your command
   authentication private key in your system keyring.
 * `TESLA_TOKEN_NAME` used to derive the entry name for your OAuth token in
   your system keyring.
 * `TESLA_KEYRING_TYPE` used override the default system keyring type for your
   OS. Run `tesla-keygen -h` to see supported values listed in the
   `-keyring-type` flag documentation. Consult [keyring
   documentation](https://github.com/99designs/keyring/#readme) for details on
   each option.
 * `TESLA_VIN` specifies a vehicle identification number. You can find your VIN
   under Controls > Software in your vehicle's UI. (Despite the name, VINs
   contain both letters and numbers).

For example:

```bash
export TESLA_KEY_NAME=$(whoami)
export TESLA_TOKEN_NAME=$(whoami)
```

At this point, you're ready to go use the [the command-line
tool](cmd/tesla-control) to start sending commands to your personal vehicle
over BLE! Alternatively, continue reading below to learn how to build an
application that can send commands over the Internet using a REST API.

## Using the HTTP proxy

This section describes how to set up and use the HTTP proxy, which allows
clients to send vehicle commands using a REST API.

As discussed above, your HTTP proxy will need to authenticate both with Tesla
(using OAuth tokens) and with individual vehicles (using a private key).

### Obtaining OAuth access tokens

Tesla's servers require your client to provide an OAuth access token before
they will forward commands to a vehicle. You must obtain the OAuth token from
the vehicle's owner. See [Tesla's
website](https://developer.tesla.com/docs/fleet-api) for instructions on
registering a developer account and obtaining OAuth tokens.

### Generating a command-authentication private key

Even if your client has a valid token, the vehicle only accepts commands that
are authorized by your client's private key.

The `tesla-keygen` utility included in this repository generates a private key,
stores it in your system keyring, and prints the corresponding public key:

```
export TESLA_KEY_NAME=$(whoami)
tesla-keygen create > public_key.pem
```

The system keyring uses your OS-dependent credential storage as the system
keyring. On macOS, for example, it defaults to using your login keychain. Run
`tesla-keygen -h` for more options.

Re-running the `tesla-keygen` command will print out the same public key
without overwriting the private key. You can force the utility to overwrite an
existing public key with `-f`.

### Distributing your public key

Vehicles verify commands using public keys. Your public key must be enrolled on
your users' vehicles before they will accept commands sent by your
application.

Here's the enrollment process from the owner's perspective:
 1. Your website or app provides a link, as described below.
 2. The user taps the link, which opens the Tesla app.
 3. The Tesla app asks the user to approve the request.
 4. If the user approves, then the Tesla app sends a command to the vehicle to
    enroll your public key. This requires the vehicle to be online and paired
    with the phone.

In order for this process to work, you must register a domain name that
identifies your application. The Tesla app will display this domain name to the
user when it asks if they wish to approve your request, and the vehicle will
display the domain name next to the key in the Locks screen.

Follow the instructions to [register your public key and
domain](https://developer.tesla.com/docs/fleet-api#register).
The public key referred to in those instructions is the `public_key.pem` file
in the above example.

Once your public key is successfully registered, provide vehicle owners with a
link to `https://tesla.com/_ak/<your_domain_name>`. For example, if you
registered `example.com`, provide a link to
`https://tesla.com/_ak/example.com`. The official Tesla iPhone or Android mobile app (version 4.27.3 or above)
will handle the rest. Customers with more than one Tesla product must select the desired vehicle before clicking
the link or scanning the QR code.

### Generating a server TLS key and certificate

The HTTP Proxy requires a TLS server certificate. For testing and development
purposes, you can create a self-signed localhost server certificate using
OpenSSL:

```
openssl req -x509 -nodes -newkey ec \
    -pkeyopt ec_paramgen_curve:secp521r1 \
    -pkeyopt ec_param_enc:named_curve  \
    -subj '/CN=localhost' \
    -keyout key.pem -out cert.pem -sha256 -days 3650 \
    -addext "extendedKeyUsage = serverAuth" \
    -addext "keyUsage = digitalSignature, keyCertSign, keyAgreement"
```

This command creates an unencrypted private key, `key.pem`.

### Running the proxy server

You can start the proxy server using the following command:

```bash
tesla-http-proxy -tls-key key.pem -cert cert.pem -port 4443
```

*Note:* In production, you'll likely want to omit the `-port 4443` and listen on
the standard port 443.

### Sending commands to the proxy server

This section illustrates how clients can reach the server using `curl`. Clients
are responsible for obtaining OAuth tokens. Obtain an OAuth token as described
as above.

Endpoints that do not support end-to-end authentication are proxied to Tesla's REST API:

```bash
export TESLA_AUTH_TOKEN=<access-token>
export VIN=<vin>
curl --cacert cert.pem \
    --header "Authorization: Bearer $TESLA_AUTH_TOKEN" \
    "https://localhost:4443/api/1/vehicles/$VIN/vehicle_data" \
    | jq -r .
```

Endpoints that support end-to-end authentication are intercepted and re-written
by the proxy, which handles session state and retries. After copying `cert.pem`
to your client, running the following command from a client will cause the
proxy to send a `flash_lights` command to the vehicle:

```bash
export TESLA_AUTH_TOKEN=<access-token>
export VIN=<vin>
curl --cacert cert.pem \
    --header 'Content-Type: application/json' \
    --header "Authorization: Bearer $TESLA_AUTH_TOKEN" \
    --data '{}' \
    "https://localhost:4443/api/1/vehicles/$VIN/command/flash_lights"
```

### REST API documentation

The HTTP proxy implements the [Tesla Fleet API vehicle command endpoints](https://developer.tesla.com/docs/fleet-api#vehicle-commands).

## Using the Golang library

You can read package [documentation on pkg.go.dev](https://pkg.go.dev/github.com/teslamotors/vehicle-command/pkg).

This repository supports `go mod` and follows [Go version
semantics](https://go.dev/doc/modules/version-numbers). Note that v0.x.x
releases do not guarantee API stability.
