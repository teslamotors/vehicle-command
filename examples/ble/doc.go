/*
Ble illustrates how to send commands to a vehicle over Bluetooth Low Energy. The program unlocks
your car and turns on the AC.

For more fleshed out examples of other commands, see
[github.com/tesla/vehicle-command/pkg/cmd/tesla-control].

# Pairing with the vehicle

To generate a key pair with OpenSSL:

	openssl ecparam -genkey -name prime256v1 -noout > private.pem
	openssl ec -in private.pem -pubout > public.pem

Next, use [github.com/teslamotors/vehicle-command/cmd/tesla-control] an send add-key request to the vehicle over BLE:

	tesla-control -vin YOUR_VIN -ble add-key-request public.pem owner cloud_key

Approve the request by tapping your NFC card or keyfob on the center console and then tapping
"Confirm" on the vehicle screen.

# Sending "unlock" and "climate on" commands

Sending commands to the vehicle requires the private key you generated above:

	./ble -vin YOUR_VIN -key private.pem

You can add the -debug flag to inspect the bytes sent over BLE.
*/
package main
