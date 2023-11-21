# Tesla Control Utility

The `tesla-control` application provides a command-line interface for sending
commands to Tesla vehicles.

This application does not run on Windows due to limitations in the available
Golang BLE packages.

## Building

Run `go get` to install Golang dependencies, and `go build` to compile.

You may also run `go install` to place `tesla-control` in your GOBIN directory.

## Key management

Commands are end-to-end authenticated, which means `tesla-control` requires
access to a private key, and the public key must be enrolled on the target
vehicle.

We'll use `tesla-keygen` to generate a key. See the root directory
[README](/README.md) file for instructions on installing this tool.

Export environment variables shared by these tools:

```bash
export TESLA_KEY_NAME=$(whoami)
export TESLA_VIN=<your Tesla's VIN>
```

Generate a private key in your system keyring, and save the public key to a file:

```
tesla-keygen create > public_key.pem
```

Now you can pair your public key with your Tesla. Get in your car, enable bluetooth
on your laptop and have your NFC card handy. Then run:

```
tesla-control -ble add-key-request public_key.pem owner cloud_key
```

The program should instruct you to confirm the new key by tapping your NFC card
on the center console.

## Sending commands

You should now be able to send commands over BLE:

```
tesla-control -ble lock
```

If you've set up your OAuth token (see [repository README file](/README.md)),
you can also send commands over the Internet:

```
tesla-control lock
```

Run `tesla-control -h` to see a full list of supported commands.
