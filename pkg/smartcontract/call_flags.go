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
