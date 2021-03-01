package compiler_test

import (
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testCase struct {
	name   string
	src    string
	result interface{}
}

// testMainIdent is a method invoked in tests by default.
const testMainIdent = "Main"

func runTestCases(t *testing.T, tcases []testCase) {
	for _, tcase := range tcases {
		t.Run(tcase.name, func(t *testing.T) { eval(t, tcase.src, tcase.result) })
	}
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
	vm.GasLimit = -1
	vm.SyscallHandler = storePlugin.syscallHandler

	b, di, err := compiler.CompileWithDebugInfo("foo.go", strings.NewReader(src))
	require.NoError(t, err)

	invokeMethod(t, testMainIdent, b, vm, di)
	return vm, storePlugin
}

func invokeMethod(t *testing.T, method string, script []byte, v *vm.VM, di *compiler.DebugInfo) {
	mainOffset := -1
	initOffset := -1
	for i := range di.Methods {
		switch di.Methods[i].ID {
		case method:
			mainOffset = int(di.Methods[i].Range.Start)
		case manifest.MethodInit:
			initOffset = int(di.Methods[i].Range.Start)
		}
	}
	require.True(t, mainOffset >= 0)
	v.LoadScriptWithFlags(script, callflag.All)
	v.Jump(v.Context(), mainOffset)
	if initOffset >= 0 {
		v.Call(v.Context(), initOffset)
	}
}

type storagePlugin struct {
	mem      map[string][]byte
	interops map[uint32]func(v *vm.VM) error
	events   []state.NotificationEvent
}

func newStoragePlugin() *storagePlugin {
	s := &storagePlugin{
		mem:      make(map[string][]byte),
		interops: make(map[uint32]func(v *vm.VM) error),
	}
	s.interops[interopnames.ToID([]byte(interopnames.SystemStorageGet))] = s.Get
	s.interops[interopnames.ToID([]byte(interopnames.SystemStoragePut))] = s.Put
	s.interops[interopnames.ToID([]byte(interopnames.SystemStorageGetContext))] = s.GetContext
	s.interops[interopnames.ToID([]byte(interopnames.SystemRuntimeNotify))] = s.Notify
	s.interops[interopnames.ToID([]byte(interopnames.SystemBinaryAtoi))] = s.Atoi
	s.interops[interopnames.ToID([]byte(interopnames.SystemBinaryItoa))] = s.Itoa
	return s

}

func (s *storagePlugin) syscallHandler(v *vm.VM, id uint32) error {
	f := s.interops[id]
	if f != nil {
		if !v.AddGas(1) {
			return errors.New("insufficient amount of gas")
		}
		return f(v)
	}
	return errors.New("syscall not found")
}

func (s *storagePlugin) Atoi(v *vm.VM) error {
	str := v.Estack().Pop().String()
	base := v.Estack().Pop().BigInt().Int64()
	n, err := strconv.ParseInt(str, int(base), 64)
	if err != nil {
		return err
	}
	v.Estack().PushVal(big.NewInt(n))
	return nil
}

func (s *storagePlugin) Itoa(v *vm.VM) error {
	n := v.Estack().Pop().BigInt()
	base := v.Estack().Pop().BigInt()
	v.Estack().PushVal(n.Text(int(base.Int64())))
	return nil
}

func (s *storagePlugin) Notify(v *vm.VM) error {
	name := v.Estack().Pop().String()
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
