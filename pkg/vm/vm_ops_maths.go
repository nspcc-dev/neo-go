package vm

import (
	"github.com/CityOfZion/neo-go/pkg/vm/stack"
)

// Add adds two stack Items together.
// Returns an error if either items cannot be casted to an integer
// or if integers cannot be added together
func Add(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	operandA, operandB, err := popTwoIntegers(ctx)
	if err != nil {
		return FAULT, err
	}
	res, err := operandA.Add(operandB)
	if err != nil {
		return FAULT, err
	}

	ctx.Estack.Push(res)

	return NONE, nil
}

// Sub subtracts two stack Items.
// Returns an error if either items cannot be casted to an integer
// or if integers cannot be subtracted together
func Sub(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	operandA, operandB, err := popTwoIntegers(ctx)
	if err != nil {
		return FAULT, err
	}
	res, err := operandB.Sub(operandA)
	if err != nil {
		return FAULT, err
	}

	ctx.Estack.Push(res)

	return NONE, nil
}

func popTwoIntegers(ctx *stack.Context) (*stack.Int, *stack.Int, error) {
	operandA, err := ctx.Estack.PopInt()
	if err != nil {
		return nil, nil, err
	}
	operandB, err := ctx.Estack.PopInt()
	if err != nil {
		return nil, nil, err
	}

	return operandA, operandB, nil
}

func popTwoByteArrays(ctx *stack.Context) (*stack.ByteArray, *stack.ByteArray, error) {
	// Pop first stack item and cast as byte array
	ba1, err := ctx.Estack.PopByteArray()
	if err != nil {
		return nil, nil, err
	}
	// Pop second stack item and cast as byte array
	ba2, err := ctx.Estack.PopByteArray()
	if err != nil {
		return nil, nil, err
	}
	return ba1, ba2, nil
}
