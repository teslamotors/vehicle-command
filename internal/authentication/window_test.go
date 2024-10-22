package authentication

import (
	"testing"
)

func TestSlidingWindow(t *testing.T) {
	type windowTest struct {
		counter                uint32
		window                 uint64
		newCounter             uint32
		expectedUpdatedCounter uint32
		expectedUpdatedWindow  uint64
		expectedOk             bool
	}
	tests := []windowTest{
		// Update should succeed because newCounter is greater than all previous counters.
		windowTest{
			counter:                100,
			window:                 uint64((1 << 0) | (1 << 5)),
			newCounter:             101,
			expectedUpdatedCounter: 101,
			expectedUpdatedWindow:  uint64(1 | (1 << 1) | (1 << 6)),
			expectedOk:             true,
		},
		// Update should succeed because newCounter is greater than all previous counters.
		// In this test, some messages were skipped and so the expectedUpdatedWindow shifts further.
		windowTest{
			counter:                100,
			window:                 uint64((1 << 0) | (1 << 5)),
			newCounter:             103,
			expectedUpdatedCounter: 103,
			expectedUpdatedWindow:  uint64((1 << 2) | (1 << 3) | (1 << 8)),
			expectedOk:             true,
		},
		// Update should succeed because newCounter is greater than all previous counters.
		// In this test, the previous counter doesn't fit in sliding window.
		windowTest{
			counter:                100,
			window:                 uint64((1 << 0) | (1 << 5)),
			newCounter:             500,
			expectedUpdatedCounter: 500,
			expectedUpdatedWindow:  0,
			expectedOk:             true,
		},
		// Update should succeed because newCounter falls in window but isn't set.
		windowTest{
			counter:                100,
			window:                 uint64((1 << 0) | (1 << 5)),
			newCounter:             98,
			expectedUpdatedCounter: 100,
			expectedUpdatedWindow:  uint64((1 << 0) | (1 << 1) | (1 << 5)),
			expectedOk:             true,
		},
		// Update should fail because newCounter falls in window and is already set.
		windowTest{
			counter:                100,
			window:                 uint64((1 << 0) | (1 << 5)),
			newCounter:             99,
			expectedUpdatedCounter: 100,
			expectedUpdatedWindow:  uint64((1 << 0) | (1 << 5)),
			expectedOk:             false,
		},
		// Update should fail because newCounter falls outside of window and freshness cannot be validated.
		windowTest{
			counter:                100,
			window:                 uint64((1 << 0) | (1 << 5)),
			newCounter:             3,
			expectedUpdatedCounter: 100,
			expectedUpdatedWindow:  uint64((1 << 0) | (1 << 5)),
			expectedOk:             false,
		},
		// Update should fail because newCounter == counter.
		windowTest{
			counter:                100,
			window:                 uint64((1 << 0) | (1 << 5)),
			newCounter:             100,
			expectedUpdatedCounter: 100,
			expectedUpdatedWindow:  uint64((1 << 0) | (1 << 5)),
			expectedOk:             false,
		},
	}
	for _, test := range tests {
		counter, window, ok := updateSlidingWindow(test.counter, test.window, test.newCounter)
		if counter != test.expectedUpdatedCounter || window != test.expectedUpdatedWindow || ok != test.expectedOk {
			t.Errorf("Failed window test %+v, got counter=%d, window=%d, ok=%v", test, counter, window, ok)
		}
		w := SlidingWindow{
			history: test.window,
			counter: test.counter,
			used:    true,
		}
		if w.Update(test.newCounter) != test.expectedOk {
			t.Errorf("Failed window test %+v", test)
		}
	}
}
