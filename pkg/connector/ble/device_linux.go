package ble

import (
	"github.com/go-ble/ble"
	"github.com/go-ble/ble/linux"
)

func newDevice() (ble.Device, error) {
	device, err := linux.NewDevice()
	if err != nil {
		return nil, err
	}
	return device, nil
}
