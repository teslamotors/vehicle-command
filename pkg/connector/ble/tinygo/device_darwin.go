package tinygo

import (
	"fmt"
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
		return nil, ErrAdapterInvalidID
	}

	return bluetooth.DefaultAdapter, nil
}

var (
	deviceCharacteristicWrite = bluetooth.DeviceCharacteristic.Write
)

func parseAddress(address string) (bluetooth.Address, error) {
	uuid, err := bluetooth.ParseUUID(address)
	if err != nil {
		return bluetooth.Address{}, fmt.Errorf("ble: failed to parse MAC address: %s", err)
	}

	return bluetooth.Address{
		UUID: uuid,
	}, nil
}
