package tinygo

import (
	"fmt"
	"strings"

	"tinygo.org/x/bluetooth"
)

func IsAdapterError(err error) bool {
	// D-Bus not found
	if strings.Contains(err.Error(), "dbus") && strings.HasSuffix(err.Error(), "no such file or directory") {
		return true
	}
	// D-Bus is running but org.bluez is not found
	if strings.Contains(err.Error(), "The name org.bluez was not provided by any .service files") {
		return true
	}
	return false
}

func AdapterErrorHelpMessage(err error) string {
	return "Failed to initialize BLE adapter: \n\t" + err.Error() + "\n" +
		"Make sure bluez and dbus are installed and running.\n" +
		"If running in a container, make sure the container has access to the host's D-Bus socket. (e.g. -v /var/run/dbus:/var/run/dbus)"
}

func newAdapter(id string) (*bluetooth.Adapter, error) {
	if id != "" {
		return bluetooth.NewAdapter(id), nil
	}

	return bluetooth.DefaultAdapter, nil
}

var (
	deviceCharacteristicWrite = bluetooth.DeviceCharacteristic.WriteWithoutResponse
)

func parseAddress(address string) (bluetooth.Address, error) {
	mac, err := bluetooth.ParseMAC(address)
	if err != nil {
		return bluetooth.Address{}, fmt.Errorf("ble: failed to parse MAC address: %s", err)
	}

	return bluetooth.Address{
		MACAddress: bluetooth.MACAddress{
			MAC: mac,
		},
	}, nil
}
