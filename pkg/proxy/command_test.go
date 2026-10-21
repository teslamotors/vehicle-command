package proxy_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/teslamotors/vehicle-command/pkg/connector"
	"github.com/teslamotors/vehicle-command/pkg/connector/inet"
	"github.com/teslamotors/vehicle-command/pkg/protocol"
	"github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/vcsec"
	"github.com/teslamotors/vehicle-command/pkg/proxy"
	"github.com/teslamotors/vehicle-command/pkg/vehicle"

	universal "github.com/teslamotors/vehicle-command/pkg/protocol/protobuf/universalmessage"
)

func TestExtractCommandAction(t *testing.T) {
	ctx := context.Background()
	params := proxy.RequestParameters{
		"volume":        5.0,
		"on":            true,
		"seat_position": 0,
		"level":         2.0,
		// Add more test cases for different commands and parameters
	}

	tests := []struct {
		command      string
		params       proxy.RequestParameters
		expectedFunc func(*vehicle.Vehicle) error
		expected     error
	}{
		{"adjust_volume", params, func(v *vehicle.Vehicle) error { return v.SetVolume(ctx, 0.0) }, nil},
		{"adjust_volume", nil, nil, &protocol.NominalError{Details: fmt.Errorf("missing volume param")}},
		{"remote_boombox", params, nil, proxy.ErrCommandNotImplemented},
		{"invalid_command", params, nil, &inet.HTTPError{Code: http.StatusBadRequest, Message: "{\"response\":null,\"error\":\"invalid_command\",\"error_description\":\"\"}"}},
	}

	for _, test := range tests {
		action, err := proxy.ExtractCommandAction(ctx, test.command, test.params)

		if errors.Is(err, test.expected) {
			if test.expected != nil && action != nil {

				t.Errorf("Expected error %#v but got action %p for command %#v", test.expected, action, test.command)
			}
		} else if err != nil && err.Error() != test.expected.Error() {
			t.Errorf("Unexpected error for command %s: %v", test.command, err)
		}
	}
}

// recordingConnector stands in for a connection to a vehicle. It records the datagrams the client
// transmits and reports AuthMethodNone so that no session handshake is needed. Nothing ever answers,
// so Send cancels the context to unblock the caller instead of waiting for a timeout.
type recordingConnector struct {
	inbox  chan []byte
	cancel context.CancelFunc

	lock sync.Mutex
	sent [][]byte
}

func (c *recordingConnector) Receive() <-chan []byte { return c.inbox }

func (c *recordingConnector) Send(_ context.Context, buffer []byte) error {
	c.lock.Lock()
	c.sent = append(c.sent, buffer)
	c.lock.Unlock()
	c.cancel()
	return nil
}

func (c *recordingConnector) VIN() string { return "5YJ3E1EA1KF000001" }

func (c *recordingConnector) Close() {}

func (c *recordingConnector) PreferredAuthMethod() connector.AuthMethod {
	return connector.AuthMethodNone
}

func (c *recordingConnector) RetryInterval() time.Duration { return time.Millisecond }

func (c *recordingConnector) AllowedLatency() time.Duration { return time.Second }

// closureMoveRequest runs the action ExtractCommandAction returns for actuate_trunk and reports the
// ClosureMoveRequest it puts on the wire. The action is an opaque closure, so reading the request is
// the only way to tell a frunk command apart from a rear trunk command.
func closureMoveRequest(t *testing.T, params proxy.RequestParameters) *vcsec.ClosureMoveRequest {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	action, err := proxy.ExtractCommandAction(ctx, "actuate_trunk", params)
	if err != nil {
		t.Fatalf("ExtractCommandAction(%v): %s", params, err)
	}

	conn := &recordingConnector{inbox: make(chan []byte), cancel: cancel}
	car, err := vehicle.NewVehicle(conn, nil, nil)
	if err != nil {
		t.Fatalf("NewVehicle: %s", err)
	}
	if err := car.Connect(ctx); err != nil {
		t.Fatalf("Connect: %s", err)
	}
	defer car.Disconnect()

	// The action always fails because the connection never answers. Only the request matters.
	_ = action(car)

	conn.lock.Lock()
	defer conn.lock.Unlock()
	if len(conn.sent) != 1 {
		t.Fatalf("transmitted %d messages, want 1", len(conn.sent))
	}

	var routable universal.RoutableMessage
	if err := proto.Unmarshal(conn.sent[0], &routable); err != nil {
		t.Fatalf("unmarshal routable message: %s", err)
	}
	var unsigned vcsec.UnsignedMessage
	if err := proto.Unmarshal(routable.GetProtobufMessageAsBytes(), &unsigned); err != nil {
		t.Fatalf("unmarshal vcsec message: %s", err)
	}
	return unsigned.GetClosureMoveRequest()
}

func TestActuateTrunkFront(t *testing.T) {
	request := closureMoveRequest(t, proxy.RequestParameters{"which_trunk": "front"})
	if request.GetFrontTrunk() != vcsec.ClosureMoveType_E_CLOSURE_MOVE_TYPE_MOVE {
		t.Errorf("frontTrunk = %s, want MOVE", request.GetFrontTrunk())
	}
	if request.GetRearTrunk() != vcsec.ClosureMoveType_E_CLOSURE_MOVE_TYPE_NONE {
		t.Errorf("rearTrunk = %s, want NONE", request.GetRearTrunk())
	}
}

func TestActuateTrunkRear(t *testing.T) {
	request := closureMoveRequest(t, proxy.RequestParameters{"which_trunk": "rear"})
	if request.GetRearTrunk() != vcsec.ClosureMoveType_E_CLOSURE_MOVE_TYPE_MOVE {
		t.Errorf("rearTrunk = %s, want MOVE", request.GetRearTrunk())
	}
	if request.GetFrontTrunk() != vcsec.ClosureMoveType_E_CLOSURE_MOVE_TYPE_NONE {
		t.Errorf("frontTrunk = %s, want NONE", request.GetFrontTrunk())
	}
}

// TestActuateTrunkInvalidParams checks that a which_trunk value the proxy cannot use is rejected
// rather than actuating the rear trunk.
func TestActuateTrunkInvalidParams(t *testing.T) {
	invalidValue := &protocol.NominalError{Details: protocol.NewError("invalid_value", false, false)}
	invalidParam := &protocol.NominalError{Details: fmt.Errorf("invalid which_trunk param")}

	tests := []struct {
		name     string
		params   proxy.RequestParameters
		expected error
	}{
		{"missing", proxy.RequestParameters{}, invalidValue},
		{"empty", proxy.RequestParameters{"which_trunk": ""}, invalidValue},
		{"unrecognized", proxy.RequestParameters{"which_trunk": "side"}, invalidValue},
		{"number", proxy.RequestParameters{"which_trunk": 42.0}, invalidParam},
		{"boolean", proxy.RequestParameters{"which_trunk": true}, invalidParam},
		{"object", proxy.RequestParameters{"which_trunk": map[string]interface{}{}}, invalidParam},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			action, err := proxy.ExtractCommandAction(context.Background(), "actuate_trunk", test.params)
			if action != nil {
				t.Errorf("got an action for %v, want none", test.params)
			}
			if err == nil {
				t.Fatalf("got no error for %v, want %s", test.params, test.expected)
			}
			if err.Error() != test.expected.Error() {
				t.Errorf("error = %q, want %q", err, test.expected)
			}
		})
	}
}
