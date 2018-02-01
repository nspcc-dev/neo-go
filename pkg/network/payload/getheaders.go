package payload

import "github.com/CityOfZion/neo-go/pkg/util"

// GetHeaders payload is the same as the "GetBlocks" payload.
type GetHeaders struct {
	HashStartStop
}

// NewGetHeaders return a pointer to a GetHeaders object.
func NewGetHeaders(start []util.Uint256, stop util.Uint256) *GetHeaders {
	p := &GetHeaders{}
	p.HashStart = start
	p.HashStop = stop

	return p
}
