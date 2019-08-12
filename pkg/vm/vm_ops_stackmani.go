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

// XDROP pops an integer n off of the stack and
// removes the n-item from the stack starting from
// the top of the stack.
func XDROP(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	n, err := ctx.Estack.PopInt()
	if err != nil {
		return FAULT, err
	}

	_, err = ctx.Estack.Remove(uint16(n.Value().Uint64()))
	if err != nil {
		return FAULT, err
	}

	return NONE, nil
}
