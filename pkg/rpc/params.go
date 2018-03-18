package rpc

type (
	// Params represent the JSON-RPC params.
	Params []interface{}
)

func (p Params) IntValueAt(index int) int {
	return p[index].(int)
}

func (p Params) FloatValueAt(index int) float64 {
	return p[index].(float64)
}

func (p Params) StringValueAt(index int) string {
	return p[index].(string)
}
