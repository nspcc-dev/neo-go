// MIT License: https://github.com/therealfakemoot/alpha/tree/dev/src/tick

package stall

import (
	"time"
)

// TimerFunc should, generally, be a closure which accepts a Timer pointer and uses enclosed values to do its work.
type TimerFunc func(*Timer)

// CleanupFunc should also be a closure, used to finalize/close values or sessions.
type CleanupFunc func(*Timer)

// Timer is a type that describes a repeating event.
//
// Using NewTimer is recommended. You may manually instantiate Timer if you want more granular control over the start process.
// Timer has three Parameters: Timer, Tf, Cf
//   Timer: a time.Ticker value, providing a signal pulse for the callback func.
//   Tf   : a TimerFunc that will be called when the Timer pulses.
//   Cf   : a CleanupFunc that is called when the timer is terminated.
type Timer struct {
	Timer *time.Ticker
	Tf    TimerFunc
	Cf    CleanupFunc
}

// Start is used to start the Timer.
func (t *Timer) Start() {
	go func() {
		for range t.Timer.C {
			t.Tf(t)
		}
	}()

}

// Done executes cleanup code.
func (t *Timer) Done() {
	t.Timer.Stop()
	t.Cf(t)
}

// NewTimer is used to create a Timer. It will automatically start the timer upon creation.
func NewTimer(d time.Duration, f TimerFunc, c CleanupFunc) *Timer {
	t := &Timer{Tf: f, Cf: c, Timer: time.NewTicker(d)}
	t.Start()
	return t
}
