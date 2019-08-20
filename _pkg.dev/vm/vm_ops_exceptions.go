package vm

import (
	"errors"

	"github.com/CityOfZion/neo-go/pkg/vm/stack"
)

// vm exceptions

// THROWIFNOT faults if the item on the top of the stack
// does not evaluate to true
// For specific logic on how a number of bytearray is evaluated can be seen
// from the boolean conversion methods on the stack items
func THROWIFNOT(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	// Pop item from top of stack
	item, err := ctx.Estack.Pop()
	if err != nil {
		return FAULT, err
	}
	// Convert to a boolean
	ok, err := item.Boolean()
	if err != nil {
		return FAULT, err
	}

	// If false, throw
	if !ok.Value() {
		return FAULT, errors.New("item on top of stack evaluates to false")
	}
	return NONE, nil
}

// THROW returns a FAULT VM state. This indicate that there is an error in the
// current context loaded program.
func THROW(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {
	return FAULT, errors.New("the execution of the script program end with an error")
}
