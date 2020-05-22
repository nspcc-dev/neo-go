package vm

import (
	"strings"

	"github.com/pkg/errors"
)

// State of the VM.
type State uint8

// Available States.
const (
	noneState State = 0
	haltState State = 1 << iota
	faultState
	breakState
)

// HasFlag checks for State flag presence.
func (s State) HasFlag(f State) bool {
	return s&f != 0
}

// String implements the stringer interface.
func (s State) String() string {
	if s == noneState {
		return "NONE"
	}

	ss := make([]string, 0, 3)
	if s.HasFlag(haltState) {
		ss = append(ss, "HALT")
	}
	if s.HasFlag(faultState) {
		ss = append(ss, "FAULT")
	}
	if s.HasFlag(breakState) {
		ss = append(ss, "BREAK")
	}
	return strings.Join(ss, ", ")
}

// StateFromString converts string into the VM State.
func StateFromString(s string) (st State, err error) {
	if s = strings.TrimSpace(s); s == "NONE" {
		return noneState, nil
	}

	ss := strings.Split(s, ",")
	for _, state := range ss {
		switch state = strings.TrimSpace(state); state {
		case "HALT":
			st |= haltState
		case "FAULT":
			st |= faultState
		case "BREAK":
			st |= breakState
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
