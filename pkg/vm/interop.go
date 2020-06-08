package vm

import (
	"errors"
	"fmt"
	"sort"

	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
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
	{emit.InteropNameToID([]byte("Neo.Runtime.Log")),
		InteropFuncPrice{runtimeLog, 1}},
	{emit.InteropNameToID([]byte("Neo.Runtime.Notify")),
		InteropFuncPrice{runtimeNotify, 1}},
	{emit.InteropNameToID([]byte("Neo.Runtime.Serialize")),
		InteropFuncPrice{RuntimeSerialize, 1}},
	{emit.InteropNameToID([]byte("System.Runtime.Serialize")),
		InteropFuncPrice{RuntimeSerialize, 1}},
	{emit.InteropNameToID([]byte("Neo.Runtime.Deserialize")),
		InteropFuncPrice{RuntimeDeserialize, 1}},
	{emit.InteropNameToID([]byte("System.Runtime.Deserialize")),
		InteropFuncPrice{RuntimeDeserialize, 1}},
	{emit.InteropNameToID([]byte("Neo.Enumerator.Create")),
		InteropFuncPrice{EnumeratorCreate, 1}},
	{emit.InteropNameToID([]byte("Neo.Enumerator.Next")),
		InteropFuncPrice{EnumeratorNext, 1}},
	{emit.InteropNameToID([]byte("Neo.Enumerator.Concat")),
		InteropFuncPrice{EnumeratorConcat, 1}},
	{emit.InteropNameToID([]byte("Neo.Enumerator.Value")),
		InteropFuncPrice{EnumeratorValue, 1}},
	{emit.InteropNameToID([]byte("Neo.Iterator.Create")),
		InteropFuncPrice{IteratorCreate, 1}},
	{emit.InteropNameToID([]byte("Neo.Iterator.Concat")),
		InteropFuncPrice{IteratorConcat, 1}},
	{emit.InteropNameToID([]byte("Neo.Iterator.Key")),
		InteropFuncPrice{IteratorKey, 1}},
	{emit.InteropNameToID([]byte("Neo.Iterator.Keys")),
		InteropFuncPrice{IteratorKeys, 1}},
	{emit.InteropNameToID([]byte("Neo.Iterator.Values")),
		InteropFuncPrice{IteratorValues, 1}},
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
	data, err := stackitem.SerializeItem(item.value)
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

	item, err := stackitem.DeserializeItem(data)
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

// EnumeratorCreate handles syscall Neo.Enumerator.Create.
func EnumeratorCreate(v *VM) error {
	data := v.Estack().Pop().Array()
	v.Estack().Push(&Element{
		value: stackitem.NewInterop(&arrayWrapper{
			index: -1,
			value: data,
		}),
	})

	return nil
}

// EnumeratorNext handles syscall Neo.Enumerator.Next.
func EnumeratorNext(v *VM) error {
	iop := v.Estack().Pop().Interop()
	arr := iop.Value().(enumerator)
	v.Estack().PushVal(arr.Next())

	return nil
}

// EnumeratorValue handles syscall Neo.Enumerator.Value.
func EnumeratorValue(v *VM) error {
	iop := v.Estack().Pop().Interop()
	arr := iop.Value().(enumerator)
	v.Estack().Push(&Element{value: arr.Value()})

	return nil
}

// EnumeratorConcat handles syscall Neo.Enumerator.Concat.
func EnumeratorConcat(v *VM) error {
	iop1 := v.Estack().Pop().Interop()
	arr1 := iop1.Value().(enumerator)
	iop2 := v.Estack().Pop().Interop()
	arr2 := iop2.Value().(enumerator)

	v.Estack().Push(&Element{
		value: stackitem.NewInterop(&concatEnum{
			current: arr1,
			second:  arr2,
		}),
	})

	return nil
}

// IteratorCreate handles syscall Neo.Iterator.Create.
func IteratorCreate(v *VM) error {
	data := v.Estack().Pop()
	var item stackitem.Item
	switch t := data.value.(type) {
	case *stackitem.Array, *stackitem.Struct:
		item = stackitem.NewInterop(&arrayWrapper{
			index: -1,
			value: t.Value().([]stackitem.Item),
		})
	case *stackitem.Map:
		item = NewMapIterator(t)
	default:
		return errors.New("non-iterable type")
	}

	v.Estack().Push(&Element{value: item})
	return nil
}

// NewMapIterator returns new interop item containing iterator over m.
func NewMapIterator(m *stackitem.Map) *stackitem.Interop {
	return stackitem.NewInterop(&mapWrapper{
		index: -1,
		m:     m.Value().([]stackitem.MapElement),
	})
}

// IteratorConcat handles syscall Neo.Iterator.Concat.
func IteratorConcat(v *VM) error {
	iop1 := v.Estack().Pop().Interop()
	iter1 := iop1.Value().(iterator)
	iop2 := v.Estack().Pop().Interop()
	iter2 := iop2.Value().(iterator)

	v.Estack().Push(&Element{value: stackitem.NewInterop(
		&concatIter{
			current: iter1,
			second:  iter2,
		},
	)})

	return nil
}

// IteratorKey handles syscall Neo.Iterator.Key.
func IteratorKey(v *VM) error {
	iop := v.estack.Pop().Interop()
	iter := iop.Value().(iterator)
	v.Estack().Push(&Element{value: iter.Key()})

	return nil
}

// IteratorKeys handles syscall Neo.Iterator.Keys.
func IteratorKeys(v *VM) error {
	iop := v.estack.Pop().Interop()
	iter := iop.Value().(iterator)
	v.Estack().Push(&Element{value: stackitem.NewInterop(
		&keysWrapper{iter},
	)})

	return nil
}

// IteratorValues handles syscall Neo.Iterator.Values.
func IteratorValues(v *VM) error {
	iop := v.estack.Pop().Interop()
	iter := iop.Value().(iterator)
	v.Estack().Push(&Element{value: stackitem.NewInterop(
		&valuesWrapper{iter},
	)})

	return nil
}
