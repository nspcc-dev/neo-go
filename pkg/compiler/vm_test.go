package compiler_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testCase struct {
	name   string
	src    string
	result interface{}
}

func runTestCases(t *testing.T, tcases []testCase) {
	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) { eval(t, tcase.src, tcase.result) })
	}
}

func evalWithoutStackChecks(t *testing.T, src string, result interface{}) {
	v := vmAndCompile(t, src)
	require.NoError(t, v.Run())
	assertResult(t, v, result)
}

func eval(t *testing.T, src string, result interface{}) {
	vm := vmAndCompile(t, src)
	err := vm.Run()
	require.NoError(t, err)
	assert.Equal(t, 1, vm.Estack().Len(), "stack contains unexpected items")
	assertResult(t, vm, result)
}

func evalWithArgs(t *testing.T, src string, op []byte, args []stackitem.Item, result interface{}) {
	vm := vmAndCompile(t, src)
	vm.LoadArgs(op, args)
	err := vm.Run()
	require.NoError(t, err)
	assert.Equal(t, 1, vm.Estack().Len(), "stack contains unexpected items")
	assertResult(t, vm, result)
}

func assertResult(t *testing.T, vm *vm.VM, result interface{}) {
	assert.Equal(t, result, vm.PopResult())
	assert.Equal(t, 0, vm.Istack().Len())
}

func vmAndCompile(t *testing.T, src string) *vm.VM {
	v, _ := vmAndCompileInterop(t, src)
	return v
}

func vmAndCompileInterop(t *testing.T, src string) (*vm.VM, *storagePlugin) {
	vm := vm.New()

	storePlugin := newStoragePlugin()
	vm.RegisterInteropGetter(storePlugin.getInterop)

	b, err := compiler.Compile(strings.NewReader(src))
	require.NoError(t, err)
	vm.Load(b)
	return vm, storePlugin
}

type storagePlugin struct {
	mem      map[string][]byte
	interops map[uint32]vm.InteropFunc
	events   []state.NotificationEvent
}

func newStoragePlugin() *storagePlugin {
	s := &storagePlugin{
		mem:      make(map[string][]byte),
		interops: make(map[uint32]vm.InteropFunc),
	}
	s.interops[emit.InteropNameToID([]byte("System.Storage.Get"))] = s.Get
	s.interops[emit.InteropNameToID([]byte("System.Storage.Put"))] = s.Put
	s.interops[emit.InteropNameToID([]byte("System.Storage.GetContext"))] = s.GetContext
	s.interops[emit.InteropNameToID([]byte("System.Runtime.Notify"))] = s.Notify
	return s

}

func (s *storagePlugin) getInterop(id uint32) *vm.InteropFuncPrice {
	f := s.interops[id]
	if f != nil {
		return &vm.InteropFuncPrice{Func: f, Price: 1}
	}
	return nil
}

func (s *storagePlugin) Notify(v *vm.VM) error {
	name := string(v.Estack().Pop().Bytes())
	item := stackitem.NewArray(v.Estack().Pop().Array())
	s.events = append(s.events, state.NotificationEvent{
		Name: name,
		Item: item,
	})
	return nil
}

func (s *storagePlugin) Delete(vm *vm.VM) error {
	vm.Estack().Pop()
	key := vm.Estack().Pop().Bytes()
	delete(s.mem, string(key))
	return nil
}

func (s *storagePlugin) Put(vm *vm.VM) error {
	vm.Estack().Pop()
	key := vm.Estack().Pop().Bytes()
	value := vm.Estack().Pop().Bytes()
	s.mem[string(key)] = value
	return nil
}

func (s *storagePlugin) Get(vm *vm.VM) error {
	vm.Estack().Pop()
	item := vm.Estack().Pop().Bytes()
	if val, ok := s.mem[string(item)]; ok {
		vm.Estack().PushVal(val)
		return nil
	}
	return fmt.Errorf("could not find %+v", item)
}

func (s *storagePlugin) GetContext(vm *vm.VM) error {
	// Pushing anything on the stack here will work. This is just to satisfy
	// the compiler, thinking it has pushed the context ^^.
	vm.Estack().PushVal(10)
	return nil
}
