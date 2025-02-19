package ble

import (
	"github.com/teslamotors/vehicle-command/internal/log"
	"tinygo.org/x/bluetooth"
)

func IsAdapterError(_ error) bool {
	// TODO: Add check for Windows
	return false
}

func AdapterErrorHelpMessage(err error) string {
	return err.Error()
}

func newAdapter(id string) *bluetooth.Adapter {
	if id != "" {
		// TODO: Add support for Windows
		log.Warning("BLE adapter ID is not supported on Windows")
	}

	return bluetooth.DefaultAdapter
}

var (
	deviceCharacteristicWrite = bluetooth.DeviceCharacteristic.WriteWithoutResponse
)

func (c *Connection) Close() {
	if err := c.device.Disconnect(); err != nil {
		log.Warning("ble: failed to disconnect: %s", err)
	}
}
