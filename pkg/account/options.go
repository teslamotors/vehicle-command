package account

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/teslamotors/vehicle-command/pkg/protocol"
)

// VehicleOption is one entry from GET /api/1/dx/vehicles/options.
// This is Tesla catalog metadata, not a signed vehicle-command field.
type VehicleOption struct {
	Code        string `json:"code"`
	DisplayName string `json:"displayName"`
	ColorCode   string `json:"colorCode,omitempty"`
	IsActive    *bool  `json:"isActive,omitempty"`
}

// GetVehicleOptions fetches Tesla's options catalog for vin.
//
// The catalog is account metadata (GET /api/1/dx/vehicles/options). Tesla often
// omits battery ($BT*) codes; this method does not invent them. See
// teslamotors/vehicle-command#391.
func (a *Account) GetVehicleOptions(ctx context.Context, vin string) ([]VehicleOption, error) {
	path, err := vehicleOptionsPath(vin)
	if err != nil {
		return nil, err
	}
	body, err := a.Get(ctx, path)
	if err != nil {
		return nil, err
	}
	return ParseVehicleOptions(body)
}

func vehicleOptionsPath(vin string) (string, error) {
	if err := validateCatalogVIN(vin); err != nil {
		return "", err
	}
	return "api/1/dx/vehicles/options?vin=" + url.QueryEscape(vin), nil
}

func validateCatalogVIN(vin string) error {
	vin = strings.TrimSpace(vin)
	if vin == "" {
		return fmt.Errorf("missing VIN")
	}
	if strings.ContainsAny(vin, "/?&# \t\n") {
		return fmt.Errorf("invalid VIN")
	}
	return nil
}

// ParseVehicleOptions decodes a Fleet API options body. Tesla has returned both
// a top-level {"codes":[...]} object (as in teslamotors/vehicle-command#391)
// and a {"response":...} envelope.
func ParseVehicleOptions(body []byte) ([]VehicleOption, error) {
	var env struct {
		Codes    []VehicleOption `json:"codes"`
		Response json.RawMessage `json:"response"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("decode vehicle options: %w", err)
	}
	if len(env.Codes) > 0 {
		return env.Codes, nil
	}
	if len(env.Response) == 0 || string(env.Response) == "null" {
		return nil, nil
	}
	var nested struct {
		Codes []VehicleOption `json:"codes"`
	}
	if err := json.Unmarshal(env.Response, &nested); err == nil && len(nested.Codes) > 0 {
		return nested.Codes, nil
	}
	var list []VehicleOption
	if err := json.Unmarshal(env.Response, &list); err == nil {
		return list, nil
	}
	return nil, fmt.Errorf("decode vehicle options: unrecognized response shape")
}

// FindBatteryOption returns the first $BT* / BT* catalog code.
// Tesla omits this field for many VINs (see teslamotors/vehicle-command#391);
// callers must not invent a code from a model option such as $MT322.
func FindBatteryOption(codes []VehicleOption) (*VehicleOption, error) {
	for i := range codes {
		if isBatteryOptionCode(codes[i].Code) {
			opt := codes[i]
			return &opt, nil
		}
	}
	return nil, protocol.ErrBatteryOptionNotInCatalog
}

func isBatteryOptionCode(code string) bool {
	code = strings.TrimSpace(code)
	code = strings.TrimPrefix(code, "$")
	if len(code) < 3 {
		return false
	}
	return strings.EqualFold(code[:2], "BT")
}
