package ble

import (
	"github.com/go-ble/ble"
	"github.com/go-ble/ble/linux"
)

func newDevice() (ble.Device, error) {
	return linux.NewDevice()
}
