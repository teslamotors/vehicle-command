package tinygo

import (
	"fmt"

	"github.com/teslamotors/vehicle-command/pkg/connector/ble"
	"tinygo.org/x/bluetooth"
)

type service struct {
	client  *bluetooth.Device
	service bluetooth.DeviceService
}

func (s *service) Rx(uuid string, callback func(buf []byte)) error {
	characteristic, err := s.discover(uuid)
	if err != nil {
		return err
	}

	if err := characteristic.EnableNotifications(callback); err != nil {
		return fmt.Errorf("ble: failed to subscribe to RX: %s", err)
	}

	return nil
}

func (s *service) Tx(uuid string) (ble.Writer, error) {
	characteristic, err := s.discover(uuid)
	if err != nil {
		return nil, err
	}

	return &writer{
		characteristic: characteristic,
		client:         s.client,
	}, nil
}

func (s *service) discover(uuid string) (bluetooth.DeviceCharacteristic, error) {
	characteristics, err := s.service.DiscoverCharacteristics([]bluetooth.UUID{mustParseUUID(uuid)})
	if err != nil {
		return bluetooth.DeviceCharacteristic{}, fmt.Errorf("ble: failed to discover service characteristics: %s", err)
	}

	if len(characteristics) == 0 {
		return bluetooth.DeviceCharacteristic{}, fmt.Errorf("ble: failed to discover service characteristics: %s", err)
	}

	return characteristics[0], nil
}

func mustParseUUID(uuid string) bluetooth.UUID {
	uuidParsed, err := bluetooth.ParseUUID(uuid)
	if err != nil {
		panic(err)
	}

	return uuidParsed
}
