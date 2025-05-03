// Package ble implements the vehicle.Connector interface using BLE.

package ble

import (
	"context"
	"crypto/sha1"
	"fmt"
	"github.com/teslamotors/vehicle-command/internal/log"
	"github.com/teslamotors/vehicle-command/pkg/connector"
	"github.com/teslamotors/vehicle-command/pkg/protocol"
	"sync"
	"time"
)

var ErrAdapterInvalidID = protocol.NewError("the bluetooth adapter ID is invalid", false, false)
var ErrMaxConnectionsExceeded = protocol.NewError("the vehicle is already connected to the maximum number of BLE devices", false, false)

var (
	mu   sync.Mutex
	impl Adapter
)

func RegisterAdapter(adapter Adapter) {
	impl = adapter
}

type ScanResult struct {
	Address     string
	LocalName   string
	RSSI        int16
	Connectable bool
}

type Adapter interface {
	InitAdapter(s *string) error
	CloseAdapter() error

	ScanVehicleBeacon(ctx context.Context, vin string) (*ScanResult, error)
	TryToConnect(ctx context.Context, vin string, target *ScanResult) (Connection, bool, error)

	IsAdapterError(err error) bool
	AdapterErrorHelpMessage(err error) string
}

type Connection interface {
	PreferredAuthMethod() connector.AuthMethod
	RetryInterval() time.Duration
	Receive() <-chan []byte
	Close()
	AllowedLatency() time.Duration
	Send(ctx context.Context, buffer []byte) error
	VIN() string
}

func VehicleLocalName(vin string) string {
	vinBytes := []byte(vin)
	digest := sha1.Sum(vinBytes)
	return fmt.Sprintf("S%02xC", digest[:8])
}

// InitAdapterWithID initializes the BLE adapter with the given ID.
// Currently, this is only supported on Linux. It is not necessary to
// call this function if using the default adapter, but if not, it
// must be called before making any other BLE calls.
// Linux:
//   - id is in the form "hciX" where X is the number of the adapter.
func InitAdapterWithID(id string) error {
	mu.Lock()
	defer mu.Unlock()
	return impl.InitAdapter(&id)
}

// CloseAdapter unsets the BLE adapter so that a new one can be created
// on the next call to InitAdapter. This does not disconnect any existing
// connections or stop any ongoing scans and must be done separately.
func CloseAdapter() error {
	mu.Lock()
	defer mu.Unlock()
	return impl.CloseAdapter()
}

func ScanVehicleBeacon(ctx context.Context, vin string) (*ScanResult, error) {
	mu.Lock()
	defer mu.Unlock()

	if err := impl.InitAdapter(nil); err != nil {
		return nil, err
	}

	a, err := impl.ScanVehicleBeacon(ctx, VehicleLocalName(vin))
	if err != nil {
		return nil, fmt.Errorf("ble: failed to scan for %s: %s", vin, err)
	}
	return a, nil
}

func NewConnection(ctx context.Context, vin string) (Connection, error) {
	return NewConnectionFromScanResult(ctx, vin, nil)
}

// NewConnectionFromScanResult creates a new BLE connection to the given target.
// If target is nil, the vehicle will be scanned for.
//
// NOTE(Linux/tinygo/bluez): If target is specified the user must make sure that the
// time between scanning and connecting is no longer than ~10 seconds as if
// it is, bluez will not allow the connection to be established until it is
// rescanned.
func NewConnectionFromScanResult(ctx context.Context, vin string, target *ScanResult) (Connection, error) {
	var lastError error
	for {
		conn, retry, err := tryToConnect(ctx, vin, target)
		if err == nil {
			return conn, nil
		}
		if !retry || impl.IsAdapterError(err) {
			return nil, err
		}
		log.Warning("BLE connection attempt failed: %s", err)
		if err := ctx.Err(); err != nil {
			if lastError != nil {
				return nil, lastError
			}
			return nil, err
		}
		lastError = err
	}
}

func IsAdapterError(err error) bool {
	return impl.IsAdapterError(err)
}

func AdapterErrorHelpMessage(err error) string {
	return impl.AdapterErrorHelpMessage(err)
}

func tryToConnect(ctx context.Context, vin string, target *ScanResult) (Connection, bool, error) {
	var err error
	mu.Lock()
	defer mu.Unlock()

	if err = impl.InitAdapter(nil); err != nil {
		return nil, false, err
	}

	localName := VehicleLocalName(vin)

	if target == nil {
		target, err = impl.ScanVehicleBeacon(ctx, localName)
		if err != nil {
			return nil, true, fmt.Errorf("ble: failed to scan for %s: %s", vin, err)
		}
	}

	if target.LocalName != localName {
		return nil, false, fmt.Errorf("ble: beacon with unexpected local name: '%s'", target.LocalName)
	}

	if !target.Connectable {
		return nil, false, ErrMaxConnectionsExceeded
	}

	return impl.TryToConnect(ctx, vin, target)
}
