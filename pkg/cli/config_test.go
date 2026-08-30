package cli_test

import (
	"github.com/teslamotors/vehicle-command/pkg/cli"
	"testing"
)

func TestDomainCLI(t *testing.T) {
	var d cli.DomainList
	if d.Set("DoesNotExist") == nil {
		t.Error("Expected error when parsing invalid domain name")
	}
	// Uppercase
	if err := d.Set("VCSEC"); err != nil {
		t.Errorf("Unexpected error when parsing VCSEC: %s", err)
	}
	// Mixed case
	if err := d.Set("infoTainMenT"); err != nil {
		t.Errorf("Unexpected error when parsing mixed-case domain name: %s", err)
	}
	if s := d.String(); s != "VCSEC,INFOTAINMENT" {
		t.Errorf("Unexpected string conversion result: %s", s)
	}
}
