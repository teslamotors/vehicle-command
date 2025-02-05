package ble

import (
	"github.com/go-ble/ble"
	"github.com/go-ble/ble/darwin"
	"github.com/teslamotors/vehicle-command/internal/log"
)

func newDevice(bdAddr ble.Addr) (ble.Device, error) {
	if bdAddr != nil || bdAddr.String() != "" {
		log.Warning("Setting the Bluetooth device address is not supported on Darwin")
	}
	device, err := darwin.NewDevice()
	if err != nil {
		return nil, err
	}
	return device, nil
}
