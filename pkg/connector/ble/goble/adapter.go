package goble

import (
	"context"

	"github.com/teslamotors/vehicle-command/pkg/connector/ble"
	"github.com/teslamotors/vehicle-command/pkg/protocol"
	goble "github.com/zlymeda/go-ble"
)

var ErrAdapterInvalidID = protocol.NewError("the bluetooth adapter ID is invalid", false, false)

func NewAdapter(id string) (ble.Adapter, error) {
	device, err := newAdapter(id)
	if err != nil {
		return nil, err
	}

	return &adapter{
		device: device,
	}, nil
}

type adapter struct {
	device goble.Device
}

func (s *adapter) ScanBeacon(ctx context.Context, name string) (*ble.Beacon, error) {
	scanCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var result *ble.Beacon

	fn := func(a goble.Advertisement) {
		if name != a.LocalName() {
			return
		}

		result = advertisementToBeacon(a)
		cancel()
	}

	err := s.device.Scan(scanCtx, false, fn)
	if err != nil && result == nil {
		return nil, err
	}

	return result, nil
}

func (s *adapter) Connect(ctx context.Context, beacon *ble.Beacon) (ble.Device, error) {
	client, err := s.device.Dial(ctx, goble.NewAddr(beacon.Address))
	if err != nil {
		return nil, err
	}

	return &device{client: client}, nil
}

func (s *adapter) Close() error {
	if s.device == nil {
		return nil
	}

	device := s.device
	s.device = nil
	return device.Stop()
}

func advertisementToBeacon(a goble.Advertisement) *ble.Beacon {
	return &ble.Beacon{
		Address:     a.Addr().String(),
		LocalName:   a.LocalName(),
		RSSI:        int16(a.RSSI()),
		Connectable: a.Connectable(),
	}
}
