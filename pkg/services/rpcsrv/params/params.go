package params

import (
	"encoding/json"
	"fmt"
)

type (
	// Params represents the JSON-RPC params.
	Params []Param
)

// FromAny allows to create Params for a slice of abstract values (by
// JSON-marshaling them).
func FromAny(arr []interface{}) (Params, error) {
	var res Params
	for i := range arr {
		b, err := json.Marshal(arr[i])
		if err != nil {
			return nil, fmt.Errorf("wrong parameter %d: %w", i, err)
		}
		res = append(res, Param{RawMessage: json.RawMessage(b)})
	}
	return res, nil
}

// Value returns the param struct for the given
// index if it exists.
func (p Params) Value(index int) *Param {
	if len(p) > index {
		return &p[index]
	}

	return nil
}

func (p Params) String() string {
	return fmt.Sprintf("%v", []Param(p))
}
