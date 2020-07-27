package callback

import (
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// PointerCallback represents callback for a pointer.
type PointerCallback struct {
	paramCount int
	offset     int
	context    *vm.Context
}

var _ Callback = (*PointerCallback)(nil)

// ArgCount implements Callback interface.
func (p *PointerCallback) ArgCount() int {
	return p.paramCount
}

// LoadContext implements Callback interface.
func (p *PointerCallback) LoadContext(v *vm.VM, args []stackitem.Item) {
	v.Call(p.context, p.offset)
	for i := len(args) - 1; i >= 0; i-- {
		v.Estack().PushVal(args[i])
	}
}

// Create creates callback using pointer and parameters count.
func Create(_ *interop.Context, v *vm.VM) error {
	ctx := v.Estack().Pop().Item().(*vm.Context)
	offset := v.Estack().Pop().Item().(*stackitem.Pointer).Position()
	count := v.Estack().Pop().BigInt().Int64()
	v.Estack().PushVal(stackitem.NewInterop(&PointerCallback{
		paramCount: int(count),
		offset:     offset,
		context:    ctx,
	}))
	return nil
}
