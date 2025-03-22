package ble

import (
	"fmt"
	"github.com/go-ble/ble"
	"github.com/go-ble/ble/linux"
	"github.com/go-ble/ble/linux/hci/cmd"
	"log"
	"time"
	"os"
	"path/filepath"
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

func newDevice() (ble.Device, error) {
    hciName, err := GetAvailableHCI()
    if err != nil {
	 return nil, fmt.Errorf("failed to find available Bluetooth adapter: %w", err)
    }

    log.Printf("Using Bluetooth adapter: %s", hciName)

    opts := []ble.Option{
	 ble.OptListenerTimeout(bleTimeout),
	 ble.OptDialerTimeout(bleTimeout),
	 ble.OptScanParams(scanParams),
	 ble.OptDeviceID(parseHCIIndex(hciName)), // Pass the detected HCI index
    }

    device, err := linux.NewDevice(opts...)
    if err != nil {
	 return nil, err
    }
    return device, nil
}

// GetAvailableHCI returns the first available HCI device (e.g., hci0, hci1, etc.)
func GetAvailableHCI() (string, error) {
    devices, err := filepath.Glob("/sys/class/bluetooth/hci*")
    if err != nil {
	 return "", fmt.Errorf("failed to list HCI devices: %v", err)
    }

    for _, device := range devices {
	 if _, err := os.Stat(device); err == nil {
	     return filepath.Base(device), nil // Return first available HCI device (e.g., "hci0")
	 }
    }

    return "", fmt.Errorf("no available HCI devices found")
}

// Extract numeric index from "hciX" (e.g., "hci1" → 1)
func parseHCIIndex(hci string) int {
    var index int
    _, err := fmt.Sscanf(hci, "hci%d", &index)
    if err != nil {
	 return 0 // Default to hci0 if parsing fails
    }
    return index
}
