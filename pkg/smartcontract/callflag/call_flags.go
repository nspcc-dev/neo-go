package callflag

// CallFlag represents call flag.
type CallFlag byte

// Default flags.
const (
	ReadStates CallFlag = 1 << iota
	WriteStates
	AllowCall
	AllowNotify

	States            = ReadStates | WriteStates
	ReadOnly          = ReadStates | AllowCall
	All               = States | AllowCall | AllowNotify
	NoneFlag CallFlag = 0
)

// Has returns true iff all bits set in cf are also set in f.
func (f CallFlag) Has(cf CallFlag) bool {
	return f&cf == cf
}
