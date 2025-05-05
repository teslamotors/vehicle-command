package iface

import "github.com/teslamotors/vehicle-command/pkg/protocol"

var ErrAdapterInvalidID = protocol.NewError("the bluetooth adapter ID is invalid", false, false)
var ErrMaxConnectionsExceeded = protocol.NewError("the vehicle is already connected to the maximum number of BLE devices", false, false)
