package vm

import (
	"errors"
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

// RuntimeSerialize handles syscalls System.Runtime.Serialize and Neo.Runtime.Serialize.
func RuntimeSerialize(vm *VM) error {
	item := vm.Estack().Pop()
	data, err := serializeItem(item.value)
	if err != nil {
		return err
	} else if len(data) > MaxItemSize {
		return errors.New("too big item")
	}

	vm.Estack().PushVal(data)

	return nil
}

// RuntimeDeserialize handles syscalls System.Runtime.Deserialize and Neo.Runtime.Deserialize.
func RuntimeDeserialize(vm *VM) error {
	data := vm.Estack().Pop().Bytes()

	item, err := deserializeItem(data)
	if err != nil {
		return err
	}

	vm.Estack().Push(&Element{value: item})

	return nil
}
