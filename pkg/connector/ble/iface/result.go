package iface

type ScanResult struct {
	Address     string
	LocalName   string
	RSSI        int16
	Connectable bool
}
