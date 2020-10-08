package request

import "fmt"

type (
	// Params represents the JSON-RPC params.
	Params []Param
)

// Value returns the param struct for the given
// index if it exists.
func (p Params) Value(index int) *Param {
	if len(p) > index {
		return &p[index]
	}

	return nil
}

// ValueWithType returns the param struct at the given index if it
// exists and matches the given type.
func (p Params) ValueWithType(index int, valType paramType) *Param {
	if val := p.Value(index); val != nil && val.Type == valType {
		return val
	}
	return nil
}

func (p Params) String() string {
	return fmt.Sprintf("%v", []Param(p))
}
