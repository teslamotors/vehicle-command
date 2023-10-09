// Example program: Use a BLE connection to unlock a vehicle and turn on the AC.

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	debugger "github.com/teslamotors/vehicle-command/internal/log"

	"github.com/teslamotors/vehicle-command/pkg/connector/ble"
	"github.com/teslamotors/vehicle-command/pkg/protocol"
	"github.com/teslamotors/vehicle-command/pkg/vehicle"
)

func main() {
	logger := log.New(os.Stderr, "", 0)
	status := 1
	debug := false
	defer func() {
		os.Exit(status)
	}()

	// Provided through command line options
	var (
		privateKeyFile string
		vin            string
	)
	flag.StringVar(&privateKeyFile, "key", "", "Private key `file` for authorizing commands (PEM PKCS8 NIST-P256)")
	flag.StringVar(&vin, "vin", "", "Vehicle Identification Number (`VIN`) of the car")
	flag.BoolVar(&debug, "debug", false, "Enable debugging of TX/RX BLE packets")
	flag.Parse()

	if debug {
		debugger.SetLevel(debugger.LevelDebug)
	}

	// For simplcity, allow 30 seconds to wake up the vehicle, connect to it,
	// and unlock. In practice you'd want a fresh timeout for each command.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if vin == "" {
		logger.Printf("Must specify VIN")
		return
	}

	var err error
	var privateKey protocol.ECDHPrivateKey
	if privateKeyFile != "" {
		if privateKey, err = protocol.LoadPrivateKey(privateKeyFile); err != nil {
			logger.Printf("Failed to load private key: %s", err)
			return
		}
	}

	conn, err := ble.NewConnection(ctx, vin)
	if err != nil {
		logger.Printf("Failed to connect to vehicle: %s", err)
		return
	}
	defer conn.Close()

	car, err := vehicle.NewVehicle(conn, privateKey, nil)
	if err != nil {
		logger.Printf("Failed to connect to vehicle: %s", err)
		return
	}

	if err := car.Connect(ctx); err != nil {
		logger.Printf("Failed to connect to vehicle: %s\n", err)
		return
	}
	defer car.Disconnect()

	// Most interactions with the car require an authenticated client.
	// StartSession() performs a handshake with the vehicle that allows
	// subsequent commands to be authenticated.
	if err := car.StartSession(ctx, nil); err != nil {
		logger.Printf("Failed to perform handshake with vehicle: %s\n", err)
		return
	}

	fmt.Println("Unlocking car...")
	if err := car.Unlock(ctx); err != nil {
		logger.Printf("Failed to unlock vehicle: %s\n", err)
		return
	}
	fmt.Println("Vehicle unlocked!")

	fmt.Println("Turning on HVAC...")
	if err := car.ClimateOn(ctx); err != nil {
		logger.Printf("Failed to turn on HVAC: %s\n", err)
		return
	}
	fmt.Println("HVAC on!")
	status = 0
}
