package ble

import (
	"github.com/go-ble/ble"
	"github.com/go-ble/ble/darwin"
)

func newDevice() (ble.Device, error) {
	device, err := darwin.NewDevice()
	if err != nil {
		return nil, err
	}
	return device, nil
}
