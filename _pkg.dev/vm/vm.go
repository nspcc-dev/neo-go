package vm

import (
	"fmt"

	"github.com/CityOfZion/neo-go/pkg/vm/stack"
)

// VM represents an instance of a Neo Virtual Machine
type VM struct {
	// ResultStack contains the results of
	// the last evaluation stack before the program terminated
	ResultStack stack.RandomAccess
	// InvocationStack contains all of the contexts
	// loaded into the vm
	InvocationStack stack.Invocation
	state           Vmstate
}

// NewVM will:
// Set the state of the VM to NONE
// instantiate a script as a new context
// Push the Context to the Invocation stack
func NewVM(script []byte) *VM {
	ctx := stack.NewContext(script)
	v := &VM{
		state: NONE,
	}
	v.InvocationStack.Push(ctx)
	return v
}

// Run loops over the current context by continuously stepping.
// Run breaks; once step returns an error or any state that is not NONE
func (v *VM) Run() (Vmstate, error) {
	for {
		state, err := v.step()
		if err != nil || state != NONE {
			return state, err
		}
	}
}

// step will read `one` opcode from the script in the current context
// Then excute that opcode
func (v *VM) step() (Vmstate, error) {
	// Get Current Context
	ctx, err := v.InvocationStack.CurrentContext()
	if err != nil {
		return FAULT, err
	}
	// Read Opcode from context
	op, _ := ctx.Next() // The only error that can occur from this, is if the pointer goes over the pointer
	// In the NEO-VM specs, this is ignored and we return the RET opcode
	// Execute OpCode
	state, err := v.executeOp(stack.Instruction(op), ctx)
	if err != nil {
		return FAULT, err
	}
	return state, nil
}

// ExecuteOp will execute one opcode on a given context.
// If the opcode is not registered, then an unknown opcode error will be returned
func (v *VM) executeOp(op stack.Instruction, ctx *stack.Context) (Vmstate, error) {
	//Find function which handles that specific opcode
	handleOp, ok := opFunc[op]
	if !ok {
		return FAULT, fmt.Errorf("unknown opcode entered %v", op)
	}
	return handleOp(op, ctx, &v.InvocationStack, &v.ResultStack)
}
