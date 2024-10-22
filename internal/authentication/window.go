package authentication

// updateSlidingWindow takes the current counter value (i.e., the highest
// counter value of any authentic message received so far), the current sliding
// window, and the newCounter value from an incoming message. The function
// returns the updated counter and window values and sets ok to true if it
// could confirm that newCounter has never been previously used. If ok is
// false, then updatedCounter = counter and updatedWindow = window.
func updateSlidingWindow(counter uint32, window uint64, newCounter uint32) (updatedCounter uint32, updatedWindow uint64, ok bool) {
	// If we exit early due to an error, we want to leave the counter/window
	// state unchanged. Therefore we initialize return values to the current
	// state.
	updatedCounter = counter
	updatedWindow = window
	ok = false

	if counter == newCounter {
		// This counter value has been used before.
		return
	}

	if newCounter < counter {
		// This message arrived out of order.
		age := counter - newCounter
		if age > windowSize {
			// Our history doesn't go back this far, so we can't determine if
			// we've seen this newCounter value before.
			return
		}
		if window>>(age-1)&1 == 1 {
			// The newCounter value has been used before.
			return
		}
		// Everything looks good.
		ok = true
		updatedWindow |= (1 << (age - 1))
		return
	}

	// If we've reached this point, newCounter > counter, so newCounter is valid.
	ok = true
	updatedCounter = newCounter
	// Compute how far we need to shift our sliding window.
	shiftCount := newCounter - counter
	updatedWindow <<= shiftCount
	// We need to set the bit in our window that corresponds to counter (if
	// newCounter = counter + 1, then this is the first [LSB] of the window).
	updatedWindow |= uint64(1) << (shiftCount - 1)
	return
}

type SlidingWindow struct {
	history uint64
	counter uint32
	used    bool
}

func (w *SlidingWindow) Update(counter uint32) bool {
	if !w.used {
		w.used = true
		w.counter = counter
		return true
	}
	var ok bool

	w.counter, w.history, ok = updateSlidingWindow(w.counter, w.history, counter)
	return ok
}
