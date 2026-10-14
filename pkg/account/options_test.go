package account

import (
	"errors"
	"testing"

	"github.com/teslamotors/vehicle-command/pkg/protocol"
)

// Sample from teslamotors/vehicle-command#391: Model 3 RWD options with no $BT* code.
const issue391OptionsJSON = `{
  "codes": [
    {
      "code": "$APBS",
      "displayName": "Autopilot",
      "isActive": true
    },
    {
      "code": "$IBB1",
      "displayName": "All Black Interior",
      "isActive": true
    },
    {
      "code": "$PPSW",
      "colorCode": "PPSW",
      "displayName": "Pearl White Multi-Coat",
      "isActive": true
    },
    {
      "code": "$W40B",
      "displayName": "18’’ Aero Wheels",
      "isActive": true
    },
    {
      "code": "$MT322",
      "displayName": "Model 3 Rear-Wheel Drive",
      "isActive": true
    },
    {
      "code": "$CW03",
      "displayName": "Cold Weather Feature",
      "isActive": true
    },
    {
      "code": "$SC04",
      "displayName": "Supercharger Network Access + Pay-as-you-go",
      "isActive": true
    }
  ]
}`

func TestParseVehicleOptionsIssue391(t *testing.T) {
	t.Parallel()
	codes, err := ParseVehicleOptions([]byte(issue391OptionsJSON))
	if err != nil {
		t.Fatal(err)
	}
	if len(codes) != 7 {
		t.Fatalf("got %d codes, want 7", len(codes))
	}
	if _, err := FindBatteryOption(codes); !errors.Is(err, protocol.ErrBatteryOptionNotInCatalog) {
		t.Fatalf("FindBatteryOption = %v, want ErrBatteryOptionNotInCatalog", err)
	}
}

func TestParseVehicleOptionsResponseEnvelope(t *testing.T) {
	t.Parallel()
	body := []byte(`{"response":{"codes":[{"code":"$BT42","displayName":"Battery","isActive":true}]}}`)
	codes, err := ParseVehicleOptions(body)
	if err != nil {
		t.Fatal(err)
	}
	opt, err := FindBatteryOption(codes)
	if err != nil {
		t.Fatal(err)
	}
	if opt.Code != "$BT42" {
		t.Fatalf("code = %q, want $BT42", opt.Code)
	}
}

func TestFindBatteryOptionVariants(t *testing.T) {
	t.Parallel()
	tests := []struct {
		code string
		want bool
	}{
		{"$BT42", true},
		{"BT37", true},
		{"$BTF0", true},
		{"$BTX6", true},
		{"$IBB1", false},
		{"$MT322", false},
		{"$APBS", false},
		{"$SC04", false},
		{"BT", false},
		{"", false},
	}
	for _, tc := range tests {
		codes := []VehicleOption{{Code: tc.code}}
		_, err := FindBatteryOption(codes)
		got := err == nil
		if got != tc.want {
			t.Errorf("FindBatteryOption(%q) present=%v, want %v (err=%v)", tc.code, got, tc.want, err)
		}
	}
}

func TestVehicleOptionsPathRejectsInjection(t *testing.T) {
	t.Parallel()
	if _, err := vehicleOptionsPath("VIN/../admin"); err == nil {
		t.Fatal("expected invalid VIN")
	}
	if _, err := vehicleOptionsPath(""); err == nil {
		t.Fatal("expected missing VIN")
	}
	path, err := vehicleOptionsPath("5YJ3E1EA1NF000001")
	if err != nil {
		t.Fatal(err)
	}
	if path != "api/1/dx/vehicles/options?vin=5YJ3E1EA1NF000001" {
		t.Fatalf("path = %q", path)
	}
}
