package vm

import (
	"github.com/CityOfZion/neo-go/pkg/vm/stack"
)

// Stack Manipulation Opcodes

// PushNBytes will Read N Bytes from the script and push it onto the stack
func PushNBytes(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	val, err := ctx.ReadBytes(int(op))
	if err != nil {
		return FAULT, err
	}
	ba := stack.NewByteArray(val)
	ctx.Estack.Push(ba)
	return NONE, nil
}

// DUPFROMALTSTACK duplicates the item on top of alternative stack and
// puts it on top of evaluation stack.
// Returns an error if the alt stack is empty.
func DUPFROMALTSTACK(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	item, err := ctx.Astack.Peek(0)
	if err != nil {
		return FAULT, err
	}

	ctx.Estack.Push(item)

	return NONE, nil
}

// TOALTSTACK  pops an item off of the evaluation stack and
// pushes it on top of the alternative stack.
// Returns an error if the alternative stack is empty.
func TOALTSTACK(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	item, err := ctx.Estack.Pop()
	if err != nil {
		return FAULT, err
	}

	ctx.Astack.Push(item)

	return NONE, nil
}

// FROMALTSTACK pops an item off of the alternative stack and
// pushes it on top of the evaluation stack.
// Returns an error if the evaluation stack is empty.
func FROMALTSTACK(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	item, err := ctx.Astack.Pop()
	if err != nil {
		return FAULT, err
	}

	ctx.Estack.Push(item)

	return NONE, nil
}

// XDROP pops an integer n off of the stack and
// removes the n-item from the stack starting from
// the top of the stack.
func XDROP(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	n, err := ctx.Estack.PopInt()
	if err != nil {
		return FAULT, err
	}

	ctx.Estack.Remove(uint16(n.Value().Uint64()))
	if err != nil {
		return FAULT, err
	}

	return NONE, nil
}
