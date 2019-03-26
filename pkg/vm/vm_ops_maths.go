package vm

import (
	"math/big"

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

// Inc increments the stack Item's value by 1.
// Returns an error if the item cannot be casted to an integer
// or if 1 cannot be added to the item
func Inc(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	i, err := ctx.Estack.PopInt()
	if err != nil {
		return FAULT, err
	}

	one, err := stack.NewInt(big.NewInt(1))
	if err != nil {
		return FAULT, err
	}

	res, err := i.Add(one)
	if err != nil {
		return FAULT, err
	}

	ctx.Estack.Push(res)

	return NONE, nil
}

// Dec decrements the stack Item's value by 1.
// Returns an error if the item cannot be casted to an integer
// or if 1 cannot be subtracted to the item
func Dec(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	i, err := ctx.Estack.PopInt()
	if err != nil {
		return FAULT, err
	}

	one, err := stack.NewInt(big.NewInt(1))
	if err != nil {
		return FAULT, err
	}

	res, err := i.Sub(one)
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
