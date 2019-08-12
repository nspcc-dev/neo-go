package vm

import (
	"github.com/CityOfZion/neo-go/pkg/vm/stack"
)

// Flow control

// RET Returns from the current context
// Returns HALT if there are nomore context's to run
func RET(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {
	_ = ctx // fix SA4009 warning

	// Pop current context from the Inovation stack
	ctx, err := istack.PopCurrentContext()
	if err != nil {
		return FAULT, err
	}
	// If this was the last context, then we copy over the evaluation stack to the resultstack
	// As the program is about to terminate, once we remove the context
	if istack.Len() == 0 {

		err = ctx.Estack.CopyTo(rstack)
		return HALT, err
	}

	return NONE, nil
}

// NOP Returns NONE VMState.
func NOP(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {
	return NONE, nil
}

// JMP moves the instruction pointer to an offset which is
// calculated base on the instructionPointerOffset method.
// Returns and error if the offset is out of range.
func JMP(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {
	offset := instructionPointerOffset(ctx)
	if err := ctx.SetIP(offset); err != nil {
		return FAULT, err

	}

	return NONE, nil
}

// JMPIF pops a boolean off of the stack and,
// if the the boolean's value is true, it
// moves the instruction pointer to an offset which is
// calculated base on the instructionPointerOffset method.
// Returns and error if the offset is out of range or
// the popped item is not a boolean.
func JMPIF(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {
	b, err := ctx.Estack.PopBoolean()
	if err != nil {
		return FAULT, err
	}

	if b.Value() {
		offset := instructionPointerOffset(ctx)
		if err := ctx.SetIP(offset); err != nil {
			return FAULT, err
		}

	}

	return NONE, nil
}

// JMPIFNOT pops a boolean off of the stack and,
// if the the boolean's value is false, it
// moves the instruction pointer to an offset which is
// calculated base on the instructionPointerOffset method.
// Returns and error if the offset is out of range or
// the popped item is not a boolean.
func JMPIFNOT(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {
	b, err := ctx.Estack.PopBoolean()
	if err != nil {
		return FAULT, err
	}

	if !b.Value() {
		offset := instructionPointerOffset(ctx)
		if err := ctx.SetIP(offset); err != nil {
			return FAULT, err
		}

	}

	return NONE, nil
}

func instructionPointerOffset(ctx *stack.Context) int {
	return ctx.IP() + int(ctx.ReadInt16()) - 3
}
