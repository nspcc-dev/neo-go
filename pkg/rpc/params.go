package rpc

type (
	// Params represents the JSON-RPC params.
	Params []Param
)

// Value returns the param struct for the given
// index if it exists.
func (p Params) Value(index int) (*Param, bool) {
	if len(p) > index {
		return &p[index], true
	}

	return nil, false
}

// ValueWithType returns the param struct at the given index if it
// exists and matches the given type.
func (p Params) ValueWithType(index int, valType paramType) (*Param, bool) {
	if val, ok := p.Value(index); ok && val.Type == valType {
		return val, true
	}

	return nil, false
}
