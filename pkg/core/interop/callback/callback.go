package callback

import (
	"errors"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// Callback is an interface for arbitrary callbacks.
type Callback interface {
	// ArgCount returns expected number of arguments.
	ArgCount() int
	// LoadContext loads context and arguments on stack.
	LoadContext(*vm.VM, []stackitem.Item)
}

// Invoke invokes provided callback.
func Invoke(ic *interop.Context, v *vm.VM) error {
	cb := v.Estack().Pop().Interop().Value().(Callback)
	args := v.Estack().Pop().Array()
	if cb.ArgCount() != len(args) {
		return errors.New("invalid argument count")
	}
	cb.LoadContext(v, args)
	switch cb.(type) {
	case *MethodCallback:
		id := emit.InteropNameToID([]byte("System.Contract.Call"))
		return ic.SyscallHandler(v, id)
	default:
		return nil
	}
}
