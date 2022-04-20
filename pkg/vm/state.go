package vm

import (
	"errors"
	"strings"
)

// State of the VM.
type State uint8

// Available States.
const (
	// HaltState represents HALT VM state.
	HaltState State = 1 << iota
	// FaultState represents FAULT VM state.
	FaultState
	// BreakState represents BREAK VM state.
	BreakState
	// NoneState represents NONE VM state.
	NoneState State = 0
)

// HasFlag checks for State flag presence.
func (s State) HasFlag(f State) bool {
	return s&f != 0
}

// String implements the stringer interface.
func (s State) String() string {
	if s == NoneState {
		return "NONE"
	}

	ss := make([]string, 0, 3)
	if s.HasFlag(HaltState) {
		ss = append(ss, "HALT")
	}
	if s.HasFlag(FaultState) {
		ss = append(ss, "FAULT")
	}
	if s.HasFlag(BreakState) {
		ss = append(ss, "BREAK")
	}
	return strings.Join(ss, ", ")
}

// StateFromString converts a string into the VM State.
func StateFromString(s string) (st State, err error) {
	if s = strings.TrimSpace(s); s == "NONE" {
		return NoneState, nil
	}

	ss := strings.Split(s, ",")
	for _, state := range ss {
		switch state = strings.TrimSpace(state); state {
		case "HALT":
			st |= HaltState
		case "FAULT":
			st |= FaultState
		case "BREAK":
			st |= BreakState
		default:
			return 0, errors.New("unknown state")
		}
	}
	return
}

// MarshalJSON implements the json.Marshaler interface.
func (s State) MarshalJSON() (data []byte, err error) {
	return []byte(`"` + s.String() + `"`), nil
}

// UnmarshalJSON implements the json.Marshaler interface.
func (s *State) UnmarshalJSON(data []byte) (err error) {
	l := len(data)
	if l < 2 || data[0] != '"' || data[l-1] != '"' {
		return errors.New("wrong format")
	}

	*s, err = StateFromString(string(data[1 : l-1]))
	return
}
