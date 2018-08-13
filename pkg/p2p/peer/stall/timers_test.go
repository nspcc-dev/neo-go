package stall

import (
	"testing"
)

func TestBasicTimer(t *testing.T) {

	var cleanup = false
	var counter = 0
	testDone := make(chan bool, 1)
	var i = 0

	f := func(to *Timer) {
		i++
		if i > 4 {
			to.Done()
		}

		counter++
	}

	cleanUp := func(to *Timer) {
		to.Timer.Stop()
		cleanup = true
		testDone <- true
	}

	NewTimer(1, f, cleanUp)

	select {
	case <-testDone:
		if counter != 5 {
			t.Errorf("Incorrect `counter` value.")
			t.Errorf("Expected: %d | Received: %d", 5, counter)
		}
	}

}

func TestTimerDone(t *testing.T) {
	var cleanup = false
	testDone := make(chan bool, 1)

	f := func(to *Timer) {
		return
	}

	c := func(to *Timer) {
		cleanup = true
		testDone <- true
	}

	timer := NewTimer(1, f, c)
	timer.Done()

	select {
	case <-testDone:
		if !cleanup {
			t.Errorf("timer.Done() failed to fire.")
		}
	}

}
