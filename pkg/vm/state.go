package vm

// State of the VM.
type State uint

// Available States.
const (
	noneState State = iota
	haltState
	faultState
	breakState
)

func (s State) String() string {
	switch s {
	case haltState:
		return "HALT"
	case faultState:
		return "FAULT"
	case breakState:
		return "BREAK"
	default:
		return "NONE"
	}
}
