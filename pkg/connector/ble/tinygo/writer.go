package tinygo

import (
	"tinygo.org/x/bluetooth"
)

type writer struct {
	characteristic bluetooth.DeviceCharacteristic
	client         *bluetooth.Device
}

func (w *writer) Write(bytes []byte) (int, error) {
	return deviceCharacteristicWrite(w.characteristic, bytes)
}

func (w *writer) MTU(_ int) (txMTU int, err error) {
	mtu, err := w.characteristic.GetMTU()
	return int(mtu), err
}
