package core

import (
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
)

// interopGasRatio is a multiplier by which a number returned from price getter
// and Fixed8 amount of GAS differ. Numbers defined in syscall tables are a multiple
// of 0.001 GAS = Fixed8(10^5).
const interopGasRatio = 100000

// StoragePrice is a price for storing 1 byte of storage.
const StoragePrice = 100000

// getPrice returns a price for executing op with the provided parameter.
// Some SYSCALLs have variable price depending on their arguments.
func getPrice(v *vm.VM, op opcode.Opcode, parameter []byte) util.Fixed8 {
	if op <= opcode.NOP {
		return 0
	}

	switch op {
	case opcode.SYSCALL:
		interopID := vm.GetInteropID(parameter)
		ifunc := v.GetInteropByID(interopID)
		if ifunc != nil && ifunc.Price > 0 {
			return toFixed8(int64(ifunc.Price))
		}
		return toFixed8(1)
	default:
		return toFixed8(1)
	}
}

func toFixed8(n int64) util.Fixed8 {
	return util.Fixed8(n * interopGasRatio)
}
