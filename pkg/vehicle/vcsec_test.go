package vehicle

import (
	"context"
	"crypto/ecdh"
	"errors"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/teslamotors/vehicle-command/pkg/protocol"

	verror "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/errors"
	universal "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/universalmessage"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/vcsec"
)

func testPublicKey() *ecdh.PublicKey {
	pkey, err := ecdh.P256().NewPublicKey([]byte{
		0x04, 0x2a, 0x01, 0xe3, 0x08, 0x84, 0x64, 0xb5, 0xe9, 0xf7, 0x2d, 0x68,
		0x79, 0x52, 0x27, 0xb2, 0xe9, 0x6b, 0xdc, 0x05, 0xb4, 0x79, 0x6d, 0xd5,
		0xa2, 0xcf, 0xc8, 0x6d, 0xa4, 0xde, 0x23, 0x37, 0xb8, 0xb2, 0xaf, 0x69,
		0x65, 0xea, 0xc9, 0x2e, 0x64, 0xc0, 0xfc, 0xdb, 0x8c, 0x5a, 0x07, 0xb7,
		0x64, 0xce, 0x6a, 0x01, 0xf4, 0x91, 0xef, 0xc5, 0x50, 0x88, 0xb5, 0xe1,
		0x98, 0x5f, 0x30, 0x4e, 0x63,
	})
	if err != nil {
		panic(err)
	}
	return pkey
}

func checkNominalError(t *testing.T, err error, expected verror.GenericError_E) {
	t.Helper()
	if err == nil {
		t.Fatalf("Expected nominal error")
	}
	var nErr *protocol.NominalVCSECError
	if errors.As(err, &nErr) {
		if nErr.Details.GetGenericError() != expected {
			t.Errorf("Expected %s but got %s", expected, nErr)
		}

		if nErr.MayHaveSucceeded() {
			t.Errorf("Expected nominal error MayHaveSucceeded() to return false")
		}

		if nErr.Temporary() {
			t.Errorf("Expected nominal error Temporary() to return false")
		}
	} else {
		t.Fatalf("Expected NominalError but got %s", err)
	}
}

func checkWhitelistOperationStatus(t *testing.T, err error, expected vcsec.WhitelistOperationInformation_E) {
	t.Helper()
	var wlErr *protocol.KeychainError
	if err == nil && expected == vcsec.WhitelistOperationInformation_E_WHITELISTOPERATION_INFORMATION_NONE {
		return
	}
	if !errors.As(err, &wlErr) {
		t.Fatalf("Expected whitelist operation error but got %v", err)
	}
	if wlErr.Code != expected {
		t.Errorf("Expected %s but got %s", expected, wlErr.Code)
	}

	if wlErr.MayHaveSucceeded() {
		t.Errorf("Expected whitelist error MayHaveSucceeded() to return false")
	}

	if wlErr.Temporary() {
		t.Errorf("Expected whitelist error Temporary() to return false")
	}
}

func TestNominalVSCECError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	vehicle, dispatch := newTestVehicle()
	if err := vehicle.StartSession(ctx, nil); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	defer vehicle.Disconnect()

	errCode := verror.GenericError_E_GENERICERROR_VEHICLE_NOT_IN_PARK

	vcsecResponse := vcsec.FromVCSECMessage{
		SubMessage: &vcsec.FromVCSECMessage_NominalError{
			NominalError: &verror.NominalError{
				GenericError: errCode,
			},
		},
	}
	rspPayload, err := proto.Marshal(&vcsecResponse)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	dispatch.fixedResponse = &universal.RoutableMessage{
		Payload: &universal.RoutableMessage_ProtobufMessageAsBytes{
			ProtobufMessageAsBytes: rspPayload,
		},
	}

	checkNominalError(t, vehicle.AddKey(ctx, testPublicKey(), true, 0), errCode)
	checkNominalError(t, vehicle.RemoveKey(ctx, testPublicKey()), errCode)
	checkNominalError(t, vehicle.Lock(ctx), errCode)
}

func TestGibberishVCSECResponse(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	vehicle, dispatch := newTestVehicle()
	if err := vehicle.StartSession(ctx, nil); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	defer vehicle.Disconnect()

	dispatch.fixedResponse = &universal.RoutableMessage{
		Payload: &universal.RoutableMessage_ProtobufMessageAsBytes{
			ProtobufMessageAsBytes: []byte{0xFF},
		},
	}

	if err := vehicle.AddKey(ctx, testPublicKey(), true, 0); !errors.Is(err, protocol.ErrBadResponse) {
		t.Errorf("Unexpected error: %s", err)
	}
	if err := vehicle.RemoveKey(ctx, testPublicKey()); !errors.Is(err, protocol.ErrBadResponse) {
		t.Errorf("Unexpected error: %s", err)
	}
	if err := vehicle.Lock(ctx); !errors.Is(err, protocol.ErrBadResponse) {
		t.Errorf("Unexpected error: %s", err)
	}
}

func (s *testSender) EnqueueVCSECBusy(t *testing.T) {
	t.Helper()
	payload := vcsec.FromVCSECMessage{
		SubMessage: &vcsec.FromVCSECMessage_CommandStatus{
			CommandStatus: &vcsec.CommandStatus{
				OperationStatus: vcsec.OperationStatus_E_OPERATIONSTATUS_WAIT,
			},
		},
	}
	encodedPayload, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	response := &universal.RoutableMessage{
		Payload: &universal.RoutableMessage_ProtobufMessageAsBytes{
			ProtobufMessageAsBytes: encodedPayload,
		},
	}
	s.EnqueueResponse(t, response)
}

func (s *testSender) EnqueueAuthenticationSuccessResponse(t *testing.T) {
	t.Helper()
	payload := vcsec.FromVCSECMessage{
		SubMessage: &vcsec.FromVCSECMessage_CommandStatus{
			CommandStatus: &vcsec.CommandStatus{
				OperationStatus: vcsec.OperationStatus_E_OPERATIONSTATUS_OK,
				SubMessage: &vcsec.CommandStatus_SignedMessageStatus{
					SignedMessageStatus: &vcsec.SignedMessageStatus{
						Counter: 1337,
					},
				},
			},
		},
	}
	encodedPayload, err := proto.Marshal(&payload)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	response := &universal.RoutableMessage{
		Payload: &universal.RoutableMessage_ProtobufMessageAsBytes{
			ProtobufMessageAsBytes: encodedPayload,
		},
	}
	s.EnqueueResponse(t, response)
}

func (s *testSender) EnqueueWhitelistOperationStatus(t *testing.T, status vcsec.WhitelistOperationInformation_E) {
	t.Helper()
	payload := vcsec.FromVCSECMessage{
		SubMessage: &vcsec.FromVCSECMessage_CommandStatus{
			CommandStatus: &vcsec.CommandStatus{
				OperationStatus: vcsec.OperationStatus_E_OPERATIONSTATUS_ERROR,
				SubMessage: &vcsec.CommandStatus_WhitelistOperationStatus{
					WhitelistOperationStatus: &vcsec.WhitelistOperationStatus{
						WhitelistOperationInformation: status,
					},
				},
			},
		},
	}
	encodedPayload, err := proto.Marshal(&payload)
	if err != nil {
		panic(err)
	}
	response := &universal.RoutableMessage{
		Payload: &universal.RoutableMessage_ProtobufMessageAsBytes{
			ProtobufMessageAsBytes: encodedPayload,
		},
	}
	s.EnqueueResponse(t, response)
}

func TestWhitelistOperationError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	vehicle, dispatch := newTestVehicle()
	if err := vehicle.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	defer vehicle.Disconnect()
	if err := vehicle.StartSession(ctx, nil); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	errCode := vcsec.WhitelistOperationInformation_E_WHITELISTOPERATION_INFORMATION_WHITELIST_FULL
	dispatch.EnqueueVCSECBusy(t)
	dispatch.EnqueueVCSECBusy(t)
	dispatch.EnqueueAuthenticationSuccessResponse(t)
	dispatch.EnqueueWhitelistOperationStatus(t, errCode)
	checkWhitelistOperationStatus(t, vehicle.AddKey(ctx, testPublicKey(), true, 0), errCode)
}
