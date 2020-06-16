package runtime

import (
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/vm"
)

// GasLeft returns remaining amount of GAS.
func GasLeft(_ *interop.Context, v *vm.VM) error {
	v.Estack().PushVal(int64(v.GasLimit - v.GasConsumed()))
	return nil
}
