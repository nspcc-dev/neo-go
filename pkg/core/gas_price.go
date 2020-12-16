package core

import (
	"github.com/nspcc-dev/neo-go/pkg/core/fee"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
)

// getPrice returns a price for executing op with the provided parameter.
func (bc *Blockchain) getPrice(v *vm.VM, op opcode.Opcode, parameter []byte) int64 {
	return fee.Opcode(bc.GetBaseExecFee(), op)
}
