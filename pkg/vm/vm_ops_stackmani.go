package vm

import (
	"math/big"

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

// XSWAP pops an integer n off of the stack and
// swaps the n-item from the stack starting from
// the top of the stack with the top stack item.
func XSWAP(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	n, err := ctx.Estack.PopInt()
	if err != nil {
		return FAULT, err
	}
	nItem, err := ctx.Estack.Peek(uint16(n.Value().Int64()))
	if err != nil {
		return FAULT, err
	}

	item, err := ctx.Estack.Peek(0)
	if err != nil {
		return FAULT, err
	}

	if err := ctx.Estack.Set(uint16(n.Value().Int64()), item); err != nil {
		return FAULT, err
	}

	if err := ctx.Estack.Set(0, nItem); err != nil {
		return FAULT, err
	}

	return NONE, nil
}

// XTUCK pops an integer n off of the stack and
// inserts the top stack item to the position len(stack)-n in the evaluation stack.
func XTUCK(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	n, err := ctx.Estack.PopInt()
	if err != nil || n.Value().Int64() < 0 {
		return FAULT, err
	}

	item, err := ctx.Estack.Peek(0)
	if err != nil {
		return FAULT, err
	}

	ras, err := ctx.Estack.Insert(uint16(n.Value().Int64()), item)
	if err != nil {
		return FAULT, err
	}

	ctx.Estack = *ras

	return NONE, nil
}

// DEPTH puts the number of stack items onto the stack.
func DEPTH(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	l := ctx.Estack.Len()
	length, err := stack.NewInt(big.NewInt(int64(l)))
	if err != nil {
		return FAULT, err
	}

	ctx.Estack.Push(length)

	return NONE, nil
}

// DROP removes the the top stack item.
// Returns error if the operation Pop cannot
// be performed.
func DROP(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	_, err := ctx.Estack.Pop()
	if err != nil {
		return FAULT, err
	}

	return NONE, nil
}
