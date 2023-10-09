package authentication

import (
	"testing"
)

func TestErrCodeString(t *testing.T) {
	err := newError(errCodeKeychainFull, "foobar")
	if err.Error() != "KeychainIsFull: foobar" {
		t.Errorf("Failed to convert error string correctly: %s", err)
	}
}
