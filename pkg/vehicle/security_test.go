package vehicle

import (
	"testing"
)

func TestValidPIN(t *testing.T) {
	validPINs := []string{
		"0000",
		"0123",
		"4569",
	}
	invalidPINs := []string{
		"",
		"123a",
		"12345",
		"1",
		"four",
	}
	for _, p := range validPINs {
		if !IsValidPIN(p) {
			t.Errorf("%s is a valid PIN", p)
		}
	}
	for _, p := range invalidPINs {
		if IsValidPIN(p) {
			t.Errorf("%s is not a valid PIN", p)
		}
	}
}
