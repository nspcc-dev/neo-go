package compiler_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
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

func eval(t *testing.T, src string, result interface{}, expectedOps ...interface{}) []byte {
	vm, _, script := vmAndCompileInterop(t, src)
	if len(expectedOps) != 0 {
		expected := io.NewBufBinWriter()
		for _, op := range expectedOps {
			switch typ := op.(type) {
			case opcode.Opcode:
				emit.Opcodes(expected.BinWriter, typ)
			case []interface{}:
				emit.Instruction(expected.BinWriter, typ[0].(opcode.Opcode), typ[1].([]byte))
			default:
				t.Fatalf("unexpected evaluation operation: %v", typ)
			}
		}

		require.Equal(t, expected.Bytes(), script)
	}
	runAndCheck(t, vm, result)
	return script
}

func evalWithError(t *testing.T, src string, e string) []byte {
	vm, _, prog := vmAndCompileInterop(t, src)
	err := vm.Run()
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), e), err)
	return prog
}

func runAndCheck(t *testing.T, v *vm.VM, result interface{}) {
	err := v.Run()
	require.NoError(t, err)
	assert.Equal(t, 1, v.Estack().Len(), "stack contains unexpected items")
	assertResult(t, v, result)
}

func evalWithArgs(t *testing.T, src string, op []byte, args []stackitem.Item, result interface{}) {
	vm := vmAndCompile(t, src)
	if len(args) > 0 {
		vm.Estack().PushVal(args)
	}
	if op != nil {
		vm.Estack().PushVal(op)
	}
	runAndCheck(t, vm, result)
}

func assertResult(t *testing.T, vm *vm.VM, result interface{}) {
	assert.Equal(t, result, vm.PopResult())
	assert.Nil(t, vm.Context())
}

func vmAndCompile(t *testing.T, src string) *vm.VM {
	v, _, _ := vmAndCompileInterop(t, src)
	return v
}

func vmAndCompileInterop(t *testing.T, src string) (*vm.VM, *storagePlugin, []byte) {
	vm := vm.New()

	storePlugin := newStoragePlugin()
	vm.GasLimit = -1
	vm.SyscallHandler = storePlugin.syscallHandler

	b, di, err := compiler.CompileWithOptions("foo.go", strings.NewReader(src), nil)
	require.NoError(t, err)

	storePlugin.info = di
	invokeMethod(t, testMainIdent, b.Script, vm, di)
	return vm, storePlugin, b.Script
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
	v.Context().Jump(mainOffset)
	if initOffset >= 0 {
		v.Call(initOffset)
	}
}

type storagePlugin struct {
	info     *compiler.DebugInfo
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
	s.interops[interopnames.ToID([]byte(interopnames.SystemRuntimeGetTime))] = s.GetTime
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

func (s *storagePlugin) GetTime(vm *vm.VM) error {
	// Pushing anything on the stack here will work. This is just to satisfy
	// the compiler, thinking it has pushed the context ^^.
	vm.Estack().PushVal(4)
	return nil
}
