package vm

import "github.com/CityOfZion/neo-go/pkg/vm/stack"

// Bitwise logic

// EQUAL pushes true to the stack
// If the two top items on the stack are equal
func EQUAL(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	itemA, itemB, err := popTwoByteArrays(ctx)
	if err != nil {
		return FAULT, err
	}
	ctx.Estack.Push(itemA.Equals(itemB))
	return NONE, nil
}

// Invert pops an integer x off of the stack and
// pushes an integer on the stack whose value
// is the bitwise complement of the value of x.
// Returns an error if the popped value is not an integer or
// if the bitwise complement cannot be taken.
func Invert(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	i, err := ctx.Estack.PopInt()
	if err != nil {
		return FAULT, err
	}

	inv, err := i.Invert()
	if err != nil {
		return FAULT, err
	}

	ctx.Estack.Push(inv)

	return NONE, nil
}

// And pops two integer off of the stack and
// pushes an integer onto the stack whose value
// is the result of the application of the bitwise AND
// operator to the two original integers' values.
// Returns an error if either items cannot be casted to an integer.
func And(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	operandA, operandB, err := popTwoIntegers(ctx)
	if err != nil {
		return FAULT, err
	}
	res, err := operandA.And(operandB)
	if err != nil {
		return FAULT, err
	}

	ctx.Estack.Push(res)

	return NONE, nil
}

// Or pops two integer off of the stack and
// pushes an integer onto the stack whose value
// is the result of the application of the bitwise OR
// operator to the two original integers' values.
// Returns an error if either items cannot be casted to an integer.
func Or(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	operandA, operandB, err := popTwoIntegers(ctx)
	if err != nil {
		return FAULT, err
	}
	res, err := operandA.Or(operandB)
	if err != nil {
		return FAULT, err
	}

	ctx.Estack.Push(res)

	return NONE, nil
}

// Xor pops two integer off of the stack and
// pushes an integer onto the stack whose value
// is the result of the application of the bitwise XOR
// operator to the two original integers' values.
// Returns an error if either items cannot be casted to an integer.
func Xor(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	operandA, operandB, err := popTwoIntegers(ctx)
	if err != nil {
		return FAULT, err
	}
	res, err := operandA.Xor(operandB)
	if err != nil {
		return FAULT, err
	}

	ctx.Estack.Push(res)

	return NONE, nil
}
