package vm

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
)

// InteropFunc allows to hook into the VM.
type InteropFunc func(vm *VM) error

// InteropFuncPrice represents an interop function with a price.
type InteropFuncPrice struct {
	Func  InteropFunc
	Price int
}

// interopIDFuncPrice adds an ID to the InteropFuncPrice.
type interopIDFuncPrice struct {
	ID uint32
	InteropFuncPrice
}

// InteropGetterFunc is a function that returns an interop function-price
// structure by the given interop ID.
type InteropGetterFunc func(uint32) *InteropFuncPrice

var defaultVMInterops = []interopIDFuncPrice{
	{InteropNameToID([]byte("Neo.Runtime.Log")),
		InteropFuncPrice{runtimeLog, 1}},
	{InteropNameToID([]byte("Neo.Runtime.Notify")),
		InteropFuncPrice{runtimeNotify, 1}},
	{InteropNameToID([]byte("Neo.Runtime.Serialize")),
		InteropFuncPrice{RuntimeSerialize, 1}},
	{InteropNameToID([]byte("System.Runtime.Serialize")),
		InteropFuncPrice{RuntimeSerialize, 1}},
	{InteropNameToID([]byte("Neo.Runtime.Deserialize")),
		InteropFuncPrice{RuntimeDeserialize, 1}},
	{InteropNameToID([]byte("System.Runtime.Deserialize")),
		InteropFuncPrice{RuntimeDeserialize, 1}},
}

func getDefaultVMInterop(id uint32) *InteropFuncPrice {
	n := sort.Search(len(defaultVMInterops), func(i int) bool {
		return defaultVMInterops[i].ID >= id
	})
	if n < len(defaultVMInterops) && defaultVMInterops[n].ID == id {
		return &defaultVMInterops[n].InteropFuncPrice
	}
	return nil
}

// InteropNameToID returns an identificator of the method based on its name.
func InteropNameToID(name []byte) uint32 {
	h := sha256.Sum256(name)
	return binary.LittleEndian.Uint32(h[:4])
}

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

// init sorts the global defaultVMInterops value.
func init() {
	sort.Slice(defaultVMInterops, func(i, j int) bool {
		return defaultVMInterops[i].ID < defaultVMInterops[j].ID
	})
}
