package tinygo

import (
	"tinygo.org/x/bluetooth"
)

func IsAdapterError(_ error) bool {
	// TODO: Add check for Windows
	return false
}

func AdapterErrorHelpMessage(err error) string {
	return err.Error()
}

func newAdapter(id string) (*bluetooth.Adapter, error) {
	if id != "" {
		// TODO: Add support for Windows
		return nil, ErrAdapterInvalidID
	}

	return bluetooth.DefaultAdapter, nil
}

var (
	deviceCharacteristicWrite = bluetooth.DeviceCharacteristic.WriteWithoutResponse
)
