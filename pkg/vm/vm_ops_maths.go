package vm

import "github.com/CityOfZion/neo-go/pkg/vm/stack"

// Add adds two stack Items together.
// Returns an error if either items cannot be casted to an integer
// or if integers cannot be added together
func Add(ctx *stack.Context, istack *stack.Invocation) error {
	operandA, err := ctx.Estack.PopInt()
	if err != nil {
		return err
	}
	operandB, err := ctx.Estack.PopInt()
	if err != nil {
		return err
	}
	res, err := operandA.Add(operandB)
	if err != nil {
		return err
	}

	ctx.Estack.Push(res)

	return nil
}
