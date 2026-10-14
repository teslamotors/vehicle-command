// Package ble implements the Connector interface using BLE.
//
// VCSEC shares a small number of BLE connections with phone keys and
// keyfobs. An enrolled client that stays connected inside the vehicle can
// prevent Walk-Away Door Lock; call Close after each command. See
// teslamotors/vehicle-command#480.
package ble
