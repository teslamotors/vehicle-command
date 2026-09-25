package vehicle

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/teslamotors/vehicle-command/pkg/protocol"
	universal "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/universalmessage"
)

func TestStateCategorySubmessages(t *testing.T) {
	for category := StateCategoryCharge; category <= StateCategoryParentalControls; category++ {
		if category.submessage() == nil {
			t.Errorf("StateCategory %d has no GetVehicleData submessage", category)
		}
	}
	if StateCategory(-1).submessage() != nil {
		t.Error("unknown StateCategory should not map to a submessage")
	}
}

func TestGetStateUnknownCategory(t *testing.T) {
	vehicle, _ := newTestVehicle()
	if _, err := vehicle.GetState(context.Background(), StateCategory(-1)); err == nil {
		t.Error("GetState accepted an unknown category")
	}
}

// Regression test for https://github.com/teslamotors/vehicle-command/issues/472.
//
// When an active navigation route makes DriveState too large for the vehicle to transmit, the
// vehicle answers with MESSAGEFAULT_ERROR_RESPONSE_MTU_EXCEEDED and no payload. GetState must
// surface that fault promptly and intact rather than retrying (the retransmitted request would
// produce the same oversized reply) or hiding the code behind a generic error.
func TestGetStateDriveResponseMTUExceeded(t *testing.T) {
	vehicle, sender := newTestVehicle()
	sender.Listen(nil)
	sender.fixedResponse = &universal.RoutableMessage{
		SignedMessageStatus: &universal.MessageStatus{
			SignedMessageFault: universal.MessageFault_E_MESSAGEFAULT_ERROR_RESPONSE_MTU_EXCEEDED,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	data, err := vehicle.GetState(ctx, StateCategoryDrive)
	elapsed := time.Since(start)

	if data != nil {
		t.Errorf("GetState returned data %v alongside an MTU fault", data)
	}
	if err == nil {
		t.Fatal("GetState returned nil error for MTU-exceeded response")
	}
	if ctx.Err() != nil {
		t.Fatalf("GetState ran until the context expired (%s); it should not retry a non-transient fault", elapsed)
	}

	var rmErr *protocol.RoutableMessageError
	if !errors.As(err, &rmErr) {
		t.Fatalf("error %T is not a *protocol.RoutableMessageError: %v", err, err)
	}
	if rmErr.Code != universal.MessageFault_E_MESSAGEFAULT_ERROR_RESPONSE_MTU_EXCEEDED {
		t.Errorf("Code = %s, want RESPONSE_MTU_EXCEEDED", rmErr.Code)
	}
	if protocol.ShouldRetry(err) {
		t.Error("ShouldRetry() = true; the same request would produce the same oversized reply")
	}
	if !protocol.MayHaveSucceeded(err) {
		t.Error("MayHaveSucceeded() = false; the vehicle did receive and process the request")
	}
	if !strings.Contains(err.Error(), "maximum message size") {
		t.Errorf("error message %q does not explain the size limit", err.Error())
	}
}
