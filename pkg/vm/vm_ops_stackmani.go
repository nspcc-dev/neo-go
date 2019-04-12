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

// DUP duplicates the top stack item.
// Returns an error if stack is empty.
func DUP(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	item, err := ctx.Estack.Peek(0)
	if err != nil {
		return FAULT, err
	}

	ctx.Estack.Push(item)

	return NONE, nil
}

// NIP removes the second top stack item.
// Returns error if the stack item contains
// only one element.
func NIP(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	_, err := ctx.Estack.Remove(1)
	if err != nil {
		return FAULT, err
	}

	return NONE, nil
}

// OVER copies the second-to-top stack item onto the top.
// Returns an error if the stack item contains
// only one element.
func OVER(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	item, err := ctx.Estack.Peek(1)
	if err != nil {
		return FAULT, err
	}

	ctx.Estack.Push(item)

	return NONE, nil
}

// PICK pops an integer n off of the stack and
// copies the n-item starting from
// the top of the stack onto the top stack item.
// Returns an error if the top stack item is not an
// integer or n-item does not exist.
func PICK(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	n, err := ctx.Estack.PopInt()
	if err != nil {
		return FAULT, err
	}

	nItem, err := ctx.Estack.Peek(uint16(n.Value().Int64()))
	if err != nil {
		return FAULT, err
	}

	ctx.Estack.Push(nItem)

	return NONE, nil
}
