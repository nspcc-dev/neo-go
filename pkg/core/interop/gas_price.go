package interop

import (
	"github.com/nspcc-dev/neo-go/pkg/core/fee"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
)

// GetPrice returns a price for executing op with the provided parameter in
// picoGAS units.
func (ic *Context) GetPrice(op opcode.Opcode, parameter []byte, args ...any) int64 {
	if true {
		if args != nil {
			return fee.OpcodeDynamic(ic.baseExecFee, op, args...)
		}
		return fee.OpcodeStatic(ic.baseExecFee, op)
	}
	return fee.Opcode(ic.baseExecFee, op)
}
