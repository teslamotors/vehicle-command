package main

import (
	"flag"
	"strings"

	goble "github.com/go-ble/ble"
	"github.com/teslamotors/vehicle-command/internal/log"
	"github.com/teslamotors/vehicle-command/pkg/connector/ble"
)

var (
	bdAddr = flag.String("bdAddr", "", "Bluetooth device address")
)

func main() {
	flag.Parse()
	log.SetLevel(log.LevelDebug)

	var err error
	if bdAddr != nil {
		log.Info("Target BLE device address: %s", *bdAddr)
		err = ble.InitDevice(goble.NewAddr(*bdAddr))
	} else {
		log.Info("Using first available BLE device")
		err = ble.InitDevice(nil)
	}

	if err != nil {
		if strings.Contains(err.Error(), "failed to find a BLE device") {
			log.Error("No BLE device found")
		} else {
			log.Error("Failed to initialize BLE device: %v", err)
		}
		return
	}

	log.Info("BLE is working")
}
