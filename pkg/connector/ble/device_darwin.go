package ble

import (
	"github.com/go-ble/ble"
	"github.com/go-ble/ble/darwin"
)

func newDevice() (ble.Device, error) {
	return darwin.NewDevice()
}
