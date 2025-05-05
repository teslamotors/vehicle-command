package iface

import "context"

type Adapter interface {
	InitAdapter(id string) error
	CloseAdapter() error

	ScanVehicleBeacon(ctx context.Context, vin string) (*ScanResult, error)
	TryToConnect(ctx context.Context, vin string, target *ScanResult) (*Connection, bool, error)

	IsAdapterError(err error) bool
	AdapterErrorHelpMessage(err error) string
}
