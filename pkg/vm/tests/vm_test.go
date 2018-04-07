package vm_test

import (
	"strings"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/vm"
	"github.com/CityOfZion/neo-go/pkg/vm/compiler"
	"github.com/stretchr/testify/assert"
)

type testCase struct {
	name   string
	src    string
	result interface{}
}

func eval(t *testing.T, src string, result interface{}) {
	vm := vmAndCompile(t, src)
	vm.Run()
	assert.Equal(t, result, vm.PopResult())
}

func evalWithArgs(t *testing.T, src string, op []byte, args []vm.StackItem, result interface{}) {
	vm := vmAndCompile(t, src)
	vm.LoadArgs(op, args)
	vm.Run()
	assert.Equal(t, result, vm.PopResult())
}

func vmAndCompile(t *testing.T, src string) *vm.VM {
	vm := vm.New(vm.ModeMute)
	b, err := compiler.Compile(strings.NewReader(src), &compiler.Options{})
	if err != nil {
		t.Fatal(err)
	}
	vm.Load(b)
	return vm
}

func TestVMAndCompilerCases(t *testing.T) {
	vm := vm.New(vm.ModeMute)

	testCases := []testCase{}
	testCases = append(testCases, numericTestCases...)
	testCases = append(testCases, assignTestCases...)
	testCases = append(testCases, binaryExprTestCases...)
	testCases = append(testCases, structTestCases...)

	for _, tc := range testCases {
		b, err := compiler.Compile(strings.NewReader(tc.src), &compiler.Options{})
		if err != nil {
			t.Fatal(err)
		}
		vm.Load(b)
		vm.Run()
		assert.Equal(t, tc.result, vm.PopResult())
	}
}
