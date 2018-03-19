package rpc

import (
	"fmt"
)

type (
	// Params represent the JSON-RPC params.
	Params []interface{}
)

func (p Params) IntValueAt(index int) int {
	data := p[index].(float64)
	return int(data)
}

func (p Params) FloatValueAt(index int) float64 {
	return p[index].(float64)
}

func (p Params) StringValueAt(index int) string {
	return p[index].(string)
}

func (p Params) ValueAt(index int) string {
	switch val := p[index].(type) {
	case int, int32, int64, float32, float64:
		return "number"
	case string:
		return "string"
	default:
		return fmt.Sprintf("%s", val)
	}
}

func (p Params) IsTypeOfValueAt(valueType string, index int) bool {
	switch p[index].(type) {
	case int, int32, int64, float32, float64:
		return valueType == "number"
	case string:
		return valueType == "string"
	default:
		return false
	}
}
