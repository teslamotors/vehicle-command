package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"strings"

	"github.com/teslamotors/vehicle-command/internal/log"
	"github.com/teslamotors/vehicle-command/pkg/connector/ble"
)

var (
	btAdapter = flag.String("btAdapter", "", "Optional ID of Bluetooth adapter to use (Linux only)")
	testScan  = flag.Bool("testScan", false, "Also test BLE scan")
)

func main() {
	flag.Parse()
	log.SetLevel(log.LevelDebug)

	var err error
	if btAdapter != nil {
		log.Info("Trying to use BLE adapter: %s", *btAdapter)
		err = ble.InitAdapterWithID(*btAdapter)
	} else {
		log.Info("Using first available BLE device")
		err = ble.InitAdapterWithID("")
	}

	if err != nil {
		if strings.Contains(err.Error(), "failed to find a BLE device") {
			log.Error("No BLE device found")
		} else {
			log.Error("Failed to initialize BLE device: %v", err)
		}
		return
	}

	log.Info("BLE adapter initialized")

	if !*testScan {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	doneChan := make(chan struct{})
	go func() {
		_, err := ble.ScanVehicleBeacon(ctx, "")
		if err != nil && ctx.Err() == nil {
			log.Error("Scan failed: %v", err)
		}
		close(doneChan)
	}()
	log.Info("Scanning for BLE devices until interrupted")

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	<-signalChan
	log.Info("Stopping scan")
	cancel()
	<-doneChan
}
