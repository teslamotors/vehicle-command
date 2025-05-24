package ble

import (
	"context"
	"io"
)

type Beacon struct {
	Address     string
	LocalName   string
	RSSI        int16
	Connectable bool
}

type Adapter interface {
	ScanBeacon(ctx context.Context, name string) (*Beacon, error)
	Connect(ctx context.Context, beacon *Beacon) (Device, error)
	Close() error
}

type Device interface {
	Service(ctx context.Context, uuid string) (Service, error)
	Close() error
}

type Service interface {
	Rx(uuid string, callback func(buf []byte)) error
	Tx(uuid string) (Writer, error)
}

type Writer interface {
	io.Writer
	MTU(rxMTU int) (txMTU int, err error)
}
