package vm

import (
	"fmt"
)

// InteropFunc allows to hook into the VM.
type InteropFunc func(vm *VM) error

// runtimeLog handles the syscall "Neo.Runtime.Log" for printing and logging stuff.
func runtimeLog(vm *VM) error {
	item := vm.Estack().Pop()
	fmt.Printf("NEO-GO-VM (log) > %s\n", item.Value())
	return nil
}

// runtimeNotify handles the syscall "Neo.Runtime.Notify" for printing and logging stuff.
func runtimeNotify(vm *VM) error {
	item := vm.Estack().Pop()
	fmt.Printf("NEO-GO-VM (notify) > %s\n", item.Value())
	return nil
}
