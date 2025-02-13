package cli

import "flag"

func (c *Config) registerCommandLineFlagsOsSpecific() {
	if c.Flags.isSet(FlagBLE) {
		flag.StringVar(&c.BtAdapterID, "bt-adapter", "", "ID of the Bluetooth adapter to use. Defaults to hci0.")
	}
}
