package vm

import (
	"fmt"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/vm/stack"
	"github.com/stretchr/testify/assert"
)

func TestPushAdd(t *testing.T) {
	builder := stack.NewBuilder()

	// PUSH TWO NUMBER
	// ADD THEM TOGETHER
	builder.EmitInt(20).EmitInt(34).EmitOpcode(stack.ADD)

	// Pass program to VM
	vm := NewVM(builder.Bytes())

	// Execute first OPCODE
	// Should be PUSH(20)
	state, err := vm.step()
	assert.Equal(t, NONE, int(state))
	assert.Nil(t, err)

	// We should have the number 20 on stack
	ok := peekTopEStackIsValue(t, vm, 20)
	assert.True(t, ok)

	// Excute second OPCODE
	// Should be PUSH(34)
	state, err = vm.step()
	assert.Equal(t, NONE, int(state))
	assert.Nil(t, err)

	// We should have the number 34 at the top of the stack
	ok = peekTopEStackIsValue(t, vm, 34)
	assert.True(t, ok)

	// Excute third OPCODE
	// Should Add both values on the stack
	state, err = vm.step()
	assert.Equal(t, NONE, int(state))
	assert.Nil(t, err)

	// We should now have one value on the stack
	//It should be equal to 20+34 = 54
	ok = EstackLen(t, vm, 1)
	assert.True(t, ok)
	ok = peekTopEStackIsValue(t, vm, 54)
	assert.True(t, ok)

	// If we try to step again, we should get a nil error and HALT
	// because we have gone over the instruction pointer
	// error is nil because when there are nomore instructions, the vm
	// will add a RET opcode and return
	state, err = vm.step()
	assert.Equal(t, HALT, int(state))
	assert.Nil(t, err)

}

func TestSimpleRun(t *testing.T) {

	// Program pushes 20 and 34 to the stack
	// Adds them together
	// pushes 54 to the stack
	// Checks if result of addition and 54 are equal
	// Faults if not

	// Push(20)
	// Push(34)
	// Add
	// Push(54)
	// Equal
	//THROWIFNOT
	builder := stack.NewBuilder()
	builder.EmitInt(20).EmitInt(34).EmitOpcode(stack.ADD)
	builder.EmitInt(54).EmitOpcode(stack.EQUAL).EmitOpcode(stack.THROWIFNOT)

	// Pass program to VM
	vm := NewVM(builder.Bytes())

	// Runs vm with program
	_, err := vm.Run()
	assert.Nil(t, err)

}

// returns true if the value at the top of the evaluation stack is a integer
// and equals the value passed in
func peekTopEStackIsValue(t *testing.T, vm *VM, value int64) bool {
	item := peakTopEstack(t, vm)
	integer, err := item.Integer()
	assert.Nil(t, err)
	return value == integer.Value().Int64()
}

// peaks the stack item on the top of the evaluation stack
// if the current context and returns it
func peakTopEstack(t *testing.T, vm *VM) stack.Item {
	ctx, err := vm.InvocationStack.CurrentContext()
	fmt.Println(err)
	assert.Nil(t, err)
	item, err := ctx.Estack.Peek(0)
	assert.Nil(t, err)
	return item
}

// returns true if the total number of items on the evaluation stack
// is equal to value
func EstackLen(t *testing.T, vm *VM, value int) bool {
	ctx, err := vm.InvocationStack.CurrentContext()
	assert.Nil(t, err)
	return value == ctx.Estack.Len()
}
