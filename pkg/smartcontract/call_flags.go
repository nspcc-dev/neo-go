package smartcontract

// CallFlag represents call flag.
type CallFlag byte

// Default flags.
const (
	NoneFlag    CallFlag = 0
	AllowStates CallFlag = 1 << iota
	AllowModifyStates
	AllowCall
	AllowNotify
	ReadOnly = AllowStates | AllowCall | AllowNotify
	All      = ReadOnly | AllowModifyStates
)

// Has returns true iff all bits set in cf are also set in f.
func (f CallFlag) Has(cf CallFlag) bool {
	return f&cf == cf
}
