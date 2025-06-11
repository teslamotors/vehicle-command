package goble

import (
	"github.com/teslamotors/vehicle-command/internal/log"
	"github.com/zlymeda/go-ble"
	"github.com/zlymeda/go-ble/darwin"
)

func IsAdapterError(_ error) bool {
	// TODO: Add check for Darwin
	return false
}

func AdapterErrorHelpMessage(err error) string {
	return err.Error()
}

func newAdapter(id string) (ble.Device, error) {
	if id != "" {
		log.Warning("Darwin does not support specifying a Bluetooth adapter ID")
		return nil, ErrAdapterInvalidID
	}
	device, err := darwin.NewDevice()
	if err != nil {
		return nil, err
	}
	return device, nil
}
