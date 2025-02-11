package ble

import (
	"tinygo.org/x/bluetooth"
)

func newAdapter(id string) *bluetooth.Adapter {
	if id != "" {
		return bluetooth.NewAdapter(id)
	}

	return bluetooth.DefaultAdapter
}
