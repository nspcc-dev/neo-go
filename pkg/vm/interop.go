package vm

import (
	"errors"
	"fmt"
	"sort"

	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
)

// interopIDFuncPrice adds an ID to the InteropFuncPrice.
type interopIDFuncPrice struct {
	ID            uint32
	Func          func(vm *VM) error
	Price         int64
	RequiredFlags callflag.CallFlag
}

var defaultVMInterops = []interopIDFuncPrice{
	{ID: interopnames.ToID([]byte(interopnames.SystemRuntimeLog)),
		Func: runtimeLog, Price: 1 << 15, RequiredFlags: callflag.AllowNotify},
	{ID: interopnames.ToID([]byte(interopnames.SystemRuntimeNotify)),
		Func: runtimeNotify, Price: 1 << 15, RequiredFlags: callflag.AllowNotify},
}

func init() {
	sort.Slice(defaultVMInterops, func(i, j int) bool { return defaultVMInterops[i].ID < defaultVMInterops[j].ID })
}

func defaultSyscallHandler(v *VM, id uint32) error {
	n := sort.Search(len(defaultVMInterops), func(i int) bool {
		return defaultVMInterops[i].ID >= id
	})
	if n >= len(defaultVMInterops) || defaultVMInterops[n].ID != id {
		return errors.New("syscall not found")
	}
	d := defaultVMInterops[n]
	ctxFlag := v.Context().sc.callFlag
	if !ctxFlag.Has(d.RequiredFlags) {
		return fmt.Errorf("missing call flags: %05b vs %05b", ctxFlag, d.RequiredFlags)
	}
	return d.Func(v)
}

// runtimeLog handles the syscall "System.Runtime.Log" for printing and logging stuff.
func runtimeLog(vm *VM) error {
	msg := vm.Estack().Pop().String()
	fmt.Printf("NEO-GO-VM (log) > %s\n", msg)
	return nil
}

// runtimeNotify handles the syscall "System.Runtime.Notify" for printing and logging stuff.
func runtimeNotify(vm *VM) error {
	name := vm.Estack().Pop().String()
	item := vm.Estack().Pop()
	fmt.Printf("NEO-GO-VM (notify) > [%s] %s\n", name, item.Value())
	return nil
}

// init sorts the global defaultVMInterops value.
func init() {
	sort.Slice(defaultVMInterops, func(i, j int) bool {
		return defaultVMInterops[i].ID < defaultVMInterops[j].ID
	})
}
