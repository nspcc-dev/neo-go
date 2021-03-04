package vm

import (
	"errors"
	"fmt"
	"sort"

	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
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
	{ID: interopnames.ToID([]byte(interopnames.SystemIteratorCreate)),
		Func: IteratorCreate, Price: 1 << 4},
	{ID: interopnames.ToID([]byte(interopnames.SystemIteratorNext)),
		Func: IteratorNext, Price: 1 << 15},
	{ID: interopnames.ToID([]byte(interopnames.SystemIteratorValue)),
		Func: IteratorValue, Price: 1 << 4},
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

// init sorts the global defaultVMInterops value.
func init() {
	sort.Slice(defaultVMInterops, func(i, j int) bool {
		return defaultVMInterops[i].ID < defaultVMInterops[j].ID
	})
}

// IteratorNext handles syscall System.Enumerator.Next.
func IteratorNext(v *VM) error {
	iop := v.Estack().Pop().Interop()
	arr := iop.Value().(iterator)
	v.Estack().PushVal(arr.Next())

	return nil
}

// IteratorValue handles syscall System.Enumerator.Value.
func IteratorValue(v *VM) error {
	iop := v.Estack().Pop().Interop()
	arr := iop.Value().(iterator)
	v.Estack().Push(&Element{value: arr.Value()})

	return nil
}

// NewIterator creates new iterator from the provided stack item.
func NewIterator(item stackitem.Item) (stackitem.Item, error) {
	switch t := item.(type) {
	case *stackitem.Array, *stackitem.Struct:
		return stackitem.NewInterop(&arrayWrapper{
			index: -1,
			value: t.Value().([]stackitem.Item),
		}), nil
	case *stackitem.Map:
		return NewMapIterator(t), nil
	default:
		data, err := t.TryBytes()
		if err != nil {
			return nil, fmt.Errorf("non-iterable type %s", t.Type())
		}
		return stackitem.NewInterop(&byteArrayWrapper{
			index: -1,
			value: data,
		}), nil
	}
}

// IteratorCreate handles syscall System.Iterator.Create.
func IteratorCreate(v *VM) error {
	data := v.Estack().Pop().Item()
	item, err := NewIterator(data)
	if err != nil {
		return err
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
