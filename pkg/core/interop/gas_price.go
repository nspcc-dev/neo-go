package interop

import (
	"github.com/nspcc-dev/neo-go/pkg/core/fee"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
)

// GetPriceV0 returns a price for executing op before Huyao hardfork, in femtoGAS units.
func (ic *Context) GetPriceV0(op opcode.Opcode, _ *vm.OpcodePriceParams) int64 {
	return fee.Opcode(ic.baseExecFee, op) * vm.OpcodePriceMultiplier
}

// GetPriceV1 returns a price for executing op since Huyao hardfork with
// the provided dynamic pricing parameters, in femtoGAS units.
func (ic *Context) GetPriceV1(op opcode.Opcode, params *vm.OpcodePriceParams) int64 {
	return fee.OpcodeV1(ic.baseExecFee, op, params)
}
