package ble

import (
	"github.com/teslamotors/vehicle-command/internal/log"
	"tinygo.org/x/bluetooth"
)

func newAdapter(id string) *bluetooth.Adapter {
	if id != "" {
		log.Warning("BLE adapter ID is not supported on Darwin")
	}

	return bluetooth.DefaultAdapter
}
