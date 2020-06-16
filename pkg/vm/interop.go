package vm

import (
	"errors"
	"fmt"
	"sort"

	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// InteropFunc allows to hook into the VM.
type InteropFunc func(vm *VM) error

// InteropFuncPrice represents an interop function with a price.
type InteropFuncPrice struct {
	Func  InteropFunc
	Price int
	// AllowedTriggers is a mask representing triggers which should be allowed by an interop.
	// 0 is interpreted as All.
	AllowedTriggers trigger.Type
	RequiredFlags   smartcontract.CallFlag
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
	{emit.InteropNameToID([]byte("System.Binary.Deserialize")),
		InteropFuncPrice{Func: RuntimeDeserialize, Price: 500000}},
	{emit.InteropNameToID([]byte("System.Binary.Serialize")),
		InteropFuncPrice{Func: RuntimeSerialize, Price: 100000}},
	{emit.InteropNameToID([]byte("System.Runtime.Log")),
		InteropFuncPrice{Func: runtimeLog, Price: 1}},
	{emit.InteropNameToID([]byte("System.Runtime.Notify")),
		InteropFuncPrice{Func: runtimeNotify, Price: 1}},
	{emit.InteropNameToID([]byte("System.Enumerator.Create")),
		InteropFuncPrice{Func: EnumeratorCreate, Price: 400}},
	{emit.InteropNameToID([]byte("System.Enumerator.Next")),
		InteropFuncPrice{Func: EnumeratorNext, Price: 1000000}},
	{emit.InteropNameToID([]byte("System.Enumerator.Concat")),
		InteropFuncPrice{Func: EnumeratorConcat, Price: 400}},
	{emit.InteropNameToID([]byte("System.Enumerator.Value")),
		InteropFuncPrice{Func: EnumeratorValue, Price: 400}},
	{emit.InteropNameToID([]byte("System.Iterator.Create")),
		InteropFuncPrice{Func: IteratorCreate, Price: 400}},
	{emit.InteropNameToID([]byte("System.Iterator.Concat")),
		InteropFuncPrice{Func: IteratorConcat, Price: 400}},
	{emit.InteropNameToID([]byte("System.Iterator.Key")),
		InteropFuncPrice{Func: IteratorKey, Price: 400}},
	{emit.InteropNameToID([]byte("System.Iterator.Keys")),
		InteropFuncPrice{Func: IteratorKeys, Price: 400}},
	{emit.InteropNameToID([]byte("System.Iterator.Values")),
		InteropFuncPrice{Func: IteratorValues, Price: 400}},
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

// runtimeLog handles the syscall "System.Runtime.Log" for printing and logging stuff.
func runtimeLog(vm *VM) error {
	item := vm.Estack().Pop()
	fmt.Printf("NEO-GO-VM (log) > %s\n", item.Value())
	return nil
}

// runtimeNotify handles the syscall "System.Runtime.Notify" for printing and logging stuff.
func runtimeNotify(vm *VM) error {
	item := vm.Estack().Pop()
	fmt.Printf("NEO-GO-VM (notify) > %s\n", item.Value())
	return nil
}

// RuntimeSerialize handles System.Binary.Serialize syscall.
func RuntimeSerialize(vm *VM) error {
	item := vm.Estack().Pop()
	data, err := stackitem.SerializeItem(item.value)
	if err != nil {
		return err
	} else if len(data) > stackitem.MaxSize {
		return errors.New("too big item")
	}

	vm.Estack().PushVal(data)

	return nil
}

// RuntimeDeserialize handles System.Binary.Deserialize syscall.
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

// EnumeratorCreate handles syscall System.Enumerator.Create.
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

// EnumeratorNext handles syscall System.Enumerator.Next.
func EnumeratorNext(v *VM) error {
	iop := v.Estack().Pop().Interop()
	arr := iop.Value().(enumerator)
	v.Estack().PushVal(arr.Next())

	return nil
}

// EnumeratorValue handles syscall System.Enumerator.Value.
func EnumeratorValue(v *VM) error {
	iop := v.Estack().Pop().Interop()
	arr := iop.Value().(enumerator)
	v.Estack().Push(&Element{value: arr.Value()})

	return nil
}

// EnumeratorConcat handles syscall System.Enumerator.Concat.
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

// IteratorCreate handles syscall System.Iterator.Create.
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

// IteratorConcat handles syscall System.Iterator.Concat.
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

// IteratorKey handles syscall System.Iterator.Key.
func IteratorKey(v *VM) error {
	iop := v.estack.Pop().Interop()
	iter := iop.Value().(iterator)
	v.Estack().Push(&Element{value: iter.Key()})

	return nil
}

// IteratorKeys handles syscall System.Iterator.Keys.
func IteratorKeys(v *VM) error {
	iop := v.estack.Pop().Interop()
	iter := iop.Value().(iterator)
	v.Estack().Push(&Element{value: stackitem.NewInterop(
		&keysWrapper{iter},
	)})

	return nil
}

// IteratorValues handles syscall System.Iterator.Values.
func IteratorValues(v *VM) error {
	iop := v.estack.Pop().Interop()
	iter := iop.Value().(iterator)
	v.Estack().Push(&Element{value: stackitem.NewInterop(
		&valuesWrapper{iter},
	)})

	return nil
}
