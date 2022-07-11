/*
Package vmstate contains a set of VM state flags along with appropriate type.
It provides a set of conversion/marshaling functions/methods for this type as
well. This package is made to make VM state reusable across all of the other
components that need it without importing whole VM package.
*/
package vmstate

import (
	"errors"
	"strings"
)

// State of the VM. It's a set of flags stored in the integer number.
type State uint8

// Available States.
const (
	// Halt represents HALT VM state (finished normally).
	Halt State = 1 << iota
	// Fault represents FAULT VM state (finished with an error).
	Fault
	// Break represents BREAK VM state (running, debug mode).
	Break
	// None represents NONE VM state (not started yet).
	None State = 0
)

// HasFlag checks for State flag presence.
func (s State) HasFlag(f State) bool {
	return s&f != 0
}

// String implements the fmt.Stringer interface.
func (s State) String() string {
	if s == None {
		return "NONE"
	}

	ss := make([]string, 0, 3)
	if s.HasFlag(Halt) {
		ss = append(ss, "HALT")
	}
	if s.HasFlag(Fault) {
		ss = append(ss, "FAULT")
	}
	if s.HasFlag(Break) {
		ss = append(ss, "BREAK")
	}
	return strings.Join(ss, ", ")
}

// FromString converts a string into the State.
func FromString(s string) (st State, err error) {
	if s = strings.TrimSpace(s); s == "NONE" {
		return None, nil
	}

	ss := strings.Split(s, ",")
	for _, state := range ss {
		switch state = strings.TrimSpace(state); state {
		case "HALT":
			st |= Halt
		case "FAULT":
			st |= Fault
		case "BREAK":
			st |= Break
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

	*s, err = FromString(string(data[1 : l-1]))
	return
}
