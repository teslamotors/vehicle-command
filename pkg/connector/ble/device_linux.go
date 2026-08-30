package ble

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-ble/ble"
	"github.com/go-ble/ble/linux"
	"github.com/go-ble/ble/linux/hci/cmd"
)

func IsAdapterError(err error) bool {
	return strings.Contains(err.Error(), "operation not permitted")
}

func AdapterErrorHelpMessage(err error) string {
	// The underlying BLE package calls HCIDEVDOWN on the BLE device, presumably as a
	// heavy-handed way of dealing with devices that are in a bad state.
	return "Failed to initialize BLE adapter: \n\t" + err.Error() + "\n" +
		"Try again after granting this application CAP_NET_ADMIN or running with root:\n\n" +
		"\tsudo setcap 'cap_net_admin=eip' \"$(which " + os.Args[0] + ")\""
}

const bleTimeout = 20 * time.Second

// TODO: Depending on the model and state, BLE advertisements come every 20ms or every 150ms.

var scanParams = cmd.LESetScanParameters{
	LEScanType:           1,    // Active scanning
	LEScanInterval:       0x10, // 10ms
	LEScanWindow:         0x10, // 10ms
	OwnAddressType:       0,    // Static
	ScanningFilterPolicy: 2,    // Basic filtered
}

func newAdapter(id *string) (ble.Device, error) {
	opts := []ble.Option{
		ble.OptDialerTimeout(bleTimeout),
		ble.OptListenerTimeout(bleTimeout),
		ble.OptScanParams(scanParams),
	}
	if id != nil && *id != "" {
		if !strings.HasPrefix(*id, "hci") {
			return nil, ErrAdapterInvalidID
		}
		hciStr := strings.TrimPrefix(*id, "hci")
		hciID, err := strconv.Atoi(hciStr)
		if err != nil || hciID < 0 || hciID > 15 {
			return nil, ErrAdapterInvalidID
		}
		opts = append(opts, ble.OptDeviceID(hciID))
	}

	device, err := linux.NewDeviceWithName("vehicle-command", opts...)
	if err != nil {
		return nil, err
	}
	return device, nil
}
