package proxy

import (
	"context"
	"errors"
	"testing"

	"github.com/teslamotors/vehicle-command/pkg/protocol"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/carserver"
)

func TestCopTempLevel(t *testing.T) {
	testCases := []struct {
		copTemp  float64
		expected carserver.ClimateState_CopActivationTemp
	}{
		{0, carserver.ClimateState_CopActivationTempLow},
		{1, carserver.ClimateState_CopActivationTempMedium},
		{2, carserver.ClimateState_CopActivationTempHigh},
	}
	for _, testCase := range testCases {
		level, err := copTempLevel(testCase.copTemp)
		if err != nil {
			t.Fatalf("copTempLevel(%v) returned error: %v", testCase.copTemp, err)
		}
		if got := carserver.ClimateState_CopActivationTemp(level); got != testCase.expected {
			t.Errorf("copTempLevel(%v) = %v, want %v", testCase.copTemp, got, testCase.expected)
		}
	}
}

func TestSetCopTempRejectsInvalidValues(t *testing.T) {
	for _, copTemp := range []float64{-1, 1.5, 3} {
		params := RequestParameters{"cop_temp": copTemp}
		_, err := ExtractCommandAction(context.Background(), "set_cop_temp", params)
		var nominalErr *protocol.NominalError
		if !errors.As(err, &nominalErr) {
			t.Errorf("set_cop_temp with cop_temp=%v: got error %v, want a NominalError", copTemp, err)
		}
	}
}
