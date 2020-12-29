package vm

import (
	"errors"
	"fmt"
	"sort"

	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// interopIDFuncPrice adds an ID to the InteropFuncPrice.
type interopIDFuncPrice struct {
	ID            uint32
	Func          func(vm *VM) error
	Price         int64
	RequiredFlags smartcontract.CallFlag
}

var defaultVMInterops = []interopIDFuncPrice{
	{ID: interopnames.ToID([]byte(interopnames.SystemBinaryDeserialize)),
		Func: RuntimeDeserialize, Price: 1 << 14},
	{ID: interopnames.ToID([]byte(interopnames.SystemBinarySerialize)),
		Func: RuntimeSerialize, Price: 1 << 12},
	{ID: interopnames.ToID([]byte(interopnames.SystemRuntimeLog)),
		Func: runtimeLog, Price: 1 << 15, RequiredFlags: smartcontract.AllowNotify},
	{ID: interopnames.ToID([]byte(interopnames.SystemRuntimeNotify)),
		Func: runtimeNotify, Price: 1 << 15, RequiredFlags: smartcontract.AllowNotify},
	{ID: interopnames.ToID([]byte(interopnames.SystemEnumeratorCreate)),
		Func: EnumeratorCreate, Price: 1 << 4},
	{ID: interopnames.ToID([]byte(interopnames.SystemEnumeratorNext)),
		Func: EnumeratorNext, Price: 1 << 15},
	{ID: interopnames.ToID([]byte(interopnames.SystemEnumeratorConcat)),
		Func: EnumeratorConcat, Price: 1 << 4},
	{ID: interopnames.ToID([]byte(interopnames.SystemEnumeratorValue)),
		Func: EnumeratorValue, Price: 1 << 4},
	{ID: interopnames.ToID([]byte(interopnames.SystemIteratorCreate)),
		Func: IteratorCreate, Price: 1 << 4},
	{ID: interopnames.ToID([]byte(interopnames.SystemIteratorConcat)),
		Func: IteratorConcat, Price: 1 << 4},
	{ID: interopnames.ToID([]byte(interopnames.SystemIteratorKey)),
		Func: IteratorKey, Price: 1 << 4},
	{ID: interopnames.ToID([]byte(interopnames.SystemIteratorKeys)),
		Func: IteratorKeys, Price: 1 << 4},
	{ID: interopnames.ToID([]byte(interopnames.SystemIteratorValues)),
		Func: IteratorValues, Price: 1 << 4},
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
	if !v.Context().callFlag.Has(d.RequiredFlags) {
		return fmt.Errorf("missing call flags: %05b vs %05b", v.Context().callFlag, d.RequiredFlags)
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
	var interop interface{}
	switch t := v.Estack().Pop().value.(type) {
	case *stackitem.Array, *stackitem.Struct:
		interop = &arrayWrapper{
			index: -1,
			value: t.Value().([]stackitem.Item),
		}
	default:
		data, err := t.TryBytes()
		if err != nil {
			return fmt.Errorf("can not create enumerator from type %s: %w", t.Type(), err)
		}
		interop = &byteArrayWrapper{
			index: -1,
			value: data,
		}
	}
	v.Estack().Push(&Element{
		value: stackitem.NewInterop(interop),
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
		data, err := t.TryBytes()
		if err != nil {
			return fmt.Errorf("non-iterable type %s", t.Type())
		}
		item = stackitem.NewInterop(&byteArrayWrapper{
			index: -1,
			value: data,
		})
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
