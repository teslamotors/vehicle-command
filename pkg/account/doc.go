// Package account implements functions for managing a Tesla account.
//
// Vehicle option codes, including battery ($BT*) identity, come from Fleet API
// GET /api/1/dx/vehicles/options (Account.GetVehicleOptions). Tesla often
// omits $BT* codes; FindBatteryOption returns
// protocol.ErrBatteryOptionNotInCatalog and does not invent a code from a
// model option. See teslamotors/vehicle-command#391.
package account
