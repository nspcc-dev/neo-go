package callback

import (
	"errors"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// SyscallCallback represents callback for a syscall.
type SyscallCallback struct {
	desc *interop.Function
}

var _ Callback = (*SyscallCallback)(nil)

// ArgCount implements Callback interface.
func (p *SyscallCallback) ArgCount() int {
	return p.desc.ParamCount
}

// LoadContext implements Callback interface.
func (p *SyscallCallback) LoadContext(v *vm.VM, args []stackitem.Item) {
	for i := len(args) - 1; i >= 0; i-- {
		v.Estack().PushVal(args[i])
	}
}

// CreateFromSyscall creates callback from syscall.
func CreateFromSyscall(ic *interop.Context, v *vm.VM) error {
	id := uint32(v.Estack().Pop().BigInt().Int64())
	f := ic.GetFunction(id)
	if f == nil {
		return errors.New("syscall not found")
	}
	if f.DisallowCallback {
		return errors.New("syscall is not allowed to be used in a callback")
	}
	v.Estack().PushVal(stackitem.NewInterop(&SyscallCallback{f}))
	return nil
}
