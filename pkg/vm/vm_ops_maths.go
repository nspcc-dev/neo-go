package vm

import (
	"github.com/CityOfZion/neo-go/pkg/vm/stack"
)

// Add adds two stack Items together.
// Returns an error if either items cannot be casted to an integer
// or if integers cannot be added together
func Add(ctx *stack.Context, istack *stack.Invocation) error {

	operandA, operandB, err := popTwoIntegers(ctx)

	res, err := operandA.Add(operandB)
	if err != nil {
		return err
	}

	ctx.Estack.Push(res)

	return nil
}

// Sub subtracts two stack Items.
// Returns an error if either items cannot be casted to an integer
// or if integers cannot be subtracted together
func Sub(ctx *stack.Context, istack *stack.Invocation) error {

	operandA, operandB, err := popTwoIntegers(ctx)

	res, err := operandB.Sub(operandA)
	if err != nil {
		return err
	}

	ctx.Estack.Push(res)

	return nil
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
