package smartcontract

// CallFlag represents call flag.
type CallFlag byte

// Default flags.
const (
	NoneFlag          CallFlag = 0
	AllowModifyStates CallFlag = 1 << iota
	AllowCall
	AllowNotify
	ReadOnly = AllowCall | AllowNotify
	All      = AllowModifyStates | AllowCall | AllowNotify
)
