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
func Invoke(ic *interop.Context) error {
	cb := ic.VM.Estack().Pop().Interop().Value().(Callback)
	args := ic.VM.Estack().Pop().Array()
	if cb.ArgCount() != len(args) {
		return errors.New("invalid argument count")
	}
	cb.LoadContext(ic.VM, args)
	switch t := cb.(type) {
	case *MethodCallback:
		id := emit.InteropNameToID([]byte("System.Contract.Call"))
		return ic.SyscallHandler(ic.VM, id)
	case *SyscallCallback:
		return ic.SyscallHandler(ic.VM, t.desc.ID)
	default:
		return nil
	}
}
