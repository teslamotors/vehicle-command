package goble

import (
	"errors"

	"github.com/zlymeda/go-ble"
)

func IsAdapterError(_ error) bool {
	// TODO: Add check for Windows
	return false
}

func AdapterErrorHelpMessage(err error) string {
	return err.Error()
}

func newAdapter(_ string) (ble.Device, error) {
	return nil, errors.New("not supported on Windows")
}
