package vm

import "strings"

// State of the VM.
type State uint

// Available States.
var (
	noneState  State = 0
	haltState  State = 1 << noneState
	faultState State = 2 << haltState
	breakState State = 1 << faultState
)

func (s State) String() string {
	if s == noneState {
		return "NONE"
	}

	states := []string{}
	switch s {
	case s & haltState:
		states = append(states, "HALT")
	case s & faultState:
		states = append(states, "FAULT")
	case s & breakState:
		states = append(states, "BREAK")
	}

	return strings.Join(states, ", ")
}
