package noderoles

//go:generate stringer -type=Role

// Role represents the type of the participant.
type Role byte

// Role enumeration.
const (
	_ Role = 1 << iota
	_
	StateValidator
	Oracle
	NeoFSAlphabet
	P2PNotary
	// last denotes the end of roles enum. Consider adding new roles before the last.
	last
)

// roles is a map of valid Role string representation to its type.
var roles map[string]Role

func init() {
	roles = make(map[string]Role)
	for i := StateValidator; i < last; i = i << 1 {
		roles[i.String()] = i
	}
}

// FromString returns a node role parsed from its string representation and a
// boolean value denoting whether the conversion was OK and the role exists.
func FromString(s string) (Role, bool) {
	r, ok := roles[s]
	return r, ok
}
