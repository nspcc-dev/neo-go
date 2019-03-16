package vm

import "github.com/CityOfZion/neo-go/pkg/vm/stack"

// Bitwise logic

// EQUAL pushes true to the stack
// If the two top items on the stack are equal
func EQUAL(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation) (Vmstate, error) {

	itemA, itemB, err := popTwoByteArrays(ctx)
	if err != nil {
		return FAULT, err
	}
	ctx.Estack.Push(itemA.Equals(itemB))
	return NONE, nil
}
