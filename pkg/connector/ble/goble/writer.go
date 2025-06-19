package goble

import "github.com/zlymeda/go-ble"

type writer struct {
	characteristic *ble.Characteristic
	client         ble.Client
}

func (w *writer) Write(bytes []byte) (int, error) {
	err := w.client.WriteCharacteristic(w.characteristic, bytes, false)
	if err != nil {
		return 0, err
	}

	return len(bytes), err
}

func (w *writer) MTU(rxMTU int) (txMTU int, err error) {
	return w.client.ExchangeMTU(rxMTU)
}
