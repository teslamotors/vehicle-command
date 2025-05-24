package goble

import (
	"bytes"
	"fmt"
	"github.com/teslamotors/vehicle-command/pkg/connector/ble"
	goble "github.com/zlymeda/go-ble"
)

type service struct {
	client  goble.Client
	service *goble.Service
}

func (s *service) Rx(uuid string, callback func(buf []byte)) error {
	characteristic, err := s.discover(uuid)
	if err != nil {
		return err
	}

	if err := s.client.Subscribe(characteristic, true, callback); err != nil {
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

func (s *service) discover(uuidStr string) (*goble.Characteristic, error) {
	uuid := goble.MustParse(uuidStr)
	characteristics, err := s.client.DiscoverCharacteristics([]goble.UUID{uuid}, s.service)
	if err != nil {
		return nil, fmt.Errorf("ble: failed to discover service characteristics: %s", err)
	}

	var characteristic *goble.Characteristic
	for _, char := range characteristics {
		if bytes.Equal(char.UUID, uuid) {
			characteristic = char
			break
		}
	}

	if characteristic == nil {
		return nil, fmt.Errorf("ble: failed to discover service characteristics: %s", err)
	}

	if _, err := s.client.DiscoverDescriptors(nil, characteristic); err != nil {
		return nil, fmt.Errorf("ble: couldn't fetch descriptors: %s", err)
	}

	return characteristic, nil
}
