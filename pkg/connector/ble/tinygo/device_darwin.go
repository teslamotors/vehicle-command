package tinygo

import (
	"github.com/teslamotors/vehicle-command/pkg/connector/ble/iface"

	"github.com/teslamotors/vehicle-command/internal/log"
	"tinygo.org/x/bluetooth"
)

func IsAdapterError(_ error) bool {
	// TODO: Add check for Darwin
	return false
}

func AdapterErrorHelpMessage(err error) string {
	return err.Error()
}

func newAdapter(id string) (*bluetooth.Adapter, error) {
	if id != "" {
		// TODO: Add support for Darwin
		return nil, iface.ErrAdapterInvalidID
	}

	return bluetooth.DefaultAdapter
}

var (
	deviceCharacteristicWrite = bluetooth.DeviceCharacteristic.Write
)

func (w *writer) Close() {
	if err := c.device.Disconnect(); err != nil {
		log.Warning("ble: failed to disconnect: %s", err)
	}
}
