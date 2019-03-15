package vm

import (
	"fmt"

	"github.com/CityOfZion/neo-go/pkg/vm/stack"
)

// VM represents an instance of a Neo Virtual Machine
type VM struct {
	InvocationStack stack.Invocation
	state           vmstate
}

//NewVM loads in a script
// uses the script to initiate a Context object
// pushes the context to the invocation stack
func NewVM(script []byte) *VM {
	ctx := stack.NewContext(script)
	v := &VM{
		state: NONE,
	}
	v.InvocationStack.Push(ctx)
	return v
}

// ExecuteOp will execute one opcode for a given context
func (v *VM) ExecuteOp(op stack.Instruction, ctx *stack.Context) error {

	handleOp, ok := opFunc[op]
	if !ok {
		return fmt.Errorf("unknown opcode entered %v", op)
	}
	err := handleOp(op, ctx, &v.InvocationStack)
	if err != nil {
		return err
	}
	return nil
}
