package smartcontract

// CallFlag represents call flag.
type CallFlag byte

// Default flags.
const (
	AllowStates CallFlag = 1 << iota
	AllowModifyStates
	AllowCall
	AllowNotify
	ReadOnly          = AllowStates | AllowCall | AllowNotify
	All               = ReadOnly | AllowModifyStates
	NoneFlag CallFlag = 0
)

// Has returns true iff all bits set in cf are also set in f.
func (f CallFlag) Has(cf CallFlag) bool {
	return f&cf == cf
}
