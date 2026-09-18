package interop

import (
	"github.com/nspcc-dev/neo-go/pkg/core/fee"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
)

// GetPriceV0 returns a price for executing op before Huyao hardfork with
// the provided parameter.
func (ic *Context) GetPriceV0(op opcode.Opcode, parameter []byte, _ *vm.OpcodePriceParams) int64 {
	return fee.Opcode(ic.baseExecFee, op)
}

// GetPriceV1 returns a price for executing op since Huyao hardfork with
// the provided parameter.
func (ic *Context) GetPriceV1(op opcode.Opcode, parameter []byte, params *vm.OpcodePriceParams) int64 {
	return fee.OpcodeV1(ic.baseExecFee, op, params)
}
