package ble

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-ble/ble"
	"github.com/go-ble/ble/linux"
	"github.com/go-ble/ble/linux/hci"
	"github.com/go-ble/ble/linux/hci/cmd"
	"github.com/teslamotors/vehicle-command/internal/log"
)

const bleTimeout = 20 * time.Second

// TODO: Depending on the model and state, BLE advertisements come every 20ms or every 150ms.

var scanParams = cmd.LESetScanParameters{
	LEScanType:           1,    // Active scanning
	LEScanInterval:       0x10, // 10ms
	LEScanWindow:         0x10, // 10ms
	OwnAddressType:       0,    // Static
	ScanningFilterPolicy: 2,    // Basic filtered
}

func newDevice(bdAddr ble.Addr) (ble.Device, error) {
	maxHciDevices := 16
	hciX := -1
	log.Debug("Scanning for HCI devices")
	bdAddrStr := ""
	if bdAddr != nil {
		bdAddrStr = bdAddr.String()
	}
	var lastErr error
	for i := 0; i < maxHciDevices; i++ {
		devHci, err := hci.NewHCI(ble.OptDeviceID(i))
		if err != nil {
			return nil, fmt.Errorf("can't create HCI %d: %v", i, err)
		}

		if err = devHci.Init(); err != nil {
			if !strings.Contains(err.Error(), "no such device") {
				lastErr = err
				log.Debug("Can't init HCI %d: %v", i, err)
			}
			continue
		}
		if err = devHci.Close(); err != nil {
			return nil, fmt.Errorf("can't close HCI %d: %v", i, err)
		}

		log.Debug("Found HCI %d: %s", i, devHci.Addr())
		if bdAddrStr == "" || devHci.Addr().String() == bdAddrStr {
			hciX = i
			break
		}
	}

	if hciX == -1 && lastErr != nil {
		return nil, lastErr
	} else if hciX == -1 {
		return nil, fmt.Errorf("no device with address %s", bdAddr)
	}
	log.Debug("Using HCI %d", hciX)

	opts := []ble.Option{
		ble.OptDeviceID(hciX),
		ble.OptListenerTimeout(bleTimeout),
		ble.OptDialerTimeout(bleTimeout),
		ble.OptScanParams(scanParams),
	}

	device, err := linux.NewDeviceWithName("vehicle-command", opts...)

	if err != nil {
		return nil, err
	}
	return device, nil
}
