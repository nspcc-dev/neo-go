package rpc

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

func (p Params) IsStringValueAt(index int) bool {
	_, ok := p[index].(string)
	return ok
}
