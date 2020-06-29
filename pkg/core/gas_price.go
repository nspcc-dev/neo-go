package core

import (
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
)

// StoragePrice is a price for storing 1 byte of storage.
const StoragePrice = 100000

// getPrice returns a price for executing op with the provided parameter.
// Some SYSCALLs have variable price depending on their arguments.
func getPrice(v *vm.VM, op opcode.Opcode, parameter []byte) int64 {
	if op == opcode.SYSCALL {
		interopID := vm.GetInteropID(parameter)
		ifunc := v.GetInteropByID(interopID)
		if ifunc != nil && ifunc.Price > 0 {
			return ifunc.Price
		}
	}
	return opcodePrice(op)
}
