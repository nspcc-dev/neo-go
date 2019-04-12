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

// ROLL pops an integer n off of the stack and
// moves the n-item starting from
// the top of the stack onto the top stack item.
// Returns an error if the top stack item is not an
// integer or n-item does not exist.
func ROLL(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	n, err := ctx.Estack.PopInt()
	if err != nil {
		return FAULT, err
	}

	nItem, err := ctx.Estack.Remove(uint16(n.Value().Int64()))
	if err != nil {
		return FAULT, err
	}

	ctx.Estack.Push(nItem)

	return NONE, nil
}

// ROT moves the third top stack item
// onto the top stack item.
// Returns an error if the third top stack item
// does not exist.
func ROT(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	item, err := ctx.Estack.Remove(2)
	if err != nil {
		return FAULT, err
	}

	ctx.Estack.Push(item)

	return NONE, nil
}

// SWAP swaps the second top stack item with
// the top stack item.
// Returns an error if the second top stack item
// does not exist.
func SWAP(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	item, err := ctx.Estack.Remove(1)
	if err != nil {
		return FAULT, err
	}

	ctx.Estack.Push(item)

	return NONE, nil
}

// TUCK copies the top stack item and
// inserts it before the second top stack item.
// Returns an error if the stack is empty or
// len(stack) is less or equal 2.
func TUCK(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	item, err := ctx.Estack.Peek(0)
	if err != nil {
		return FAULT, err
	}

	ras, err := ctx.Estack.Insert(2, item)
	if err != nil {
		return FAULT, err
	}
	ctx.Estack = *ras

	return NONE, nil
}
