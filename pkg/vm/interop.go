package vm

import (
	"fmt"
)

// InteropFunc allows to hook into the VM.
type InteropFunc func(vm *VM) error

// runtimeLog will handle the syscall "Neo.Runtime.Log" for printing and logging stuff.
func runtimeLog(vm *VM) error {
	item := vm.Estack().Pop()
	fmt.Printf("NEO-GO-VM (log) > %s\n", item.value.Value())
	return nil
}

// runtimeNotify will handle the syscall "Neo.Runtime.Notify" for printing and logging stuff.
func runtimeNotify(vm *VM) error {
	item := vm.Estack().Pop()
	fmt.Printf("NEO-GO-VM (notify) > %s\n", item.value.Value())
	return nil
}
