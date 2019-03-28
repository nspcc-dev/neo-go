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
// or if 1 cannot be added to the item.
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
// or if 1 cannot be subtracted to the item.
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

// Nz pops an integer from the stack.
// Then pushes a boolean to the stack which evaluates to true
// iff the integer was not zero.
// Returns an error if the popped item cannot be casted to an integer
// or if we cannot create a boolean.
func Nz(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	i, err := ctx.Estack.PopInt()
	if err != nil {
		return FAULT, err
	}

	b, err := i.Boolean()
	if err != nil {
		return FAULT, err
	}

	ctx.Estack.Push(b)

	return NONE, nil
}

// Mul multiplies two stack Items together.
// Returns an error if either items cannot be casted to an integer
// or if integers cannot be multiplied together.
func Mul(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	operandA, operandB, err := popTwoIntegers(ctx)
	if err != nil {
		return FAULT, err
	}
	res, err := operandA.Mul(operandB)
	if err != nil {
		return FAULT, err
	}

	ctx.Estack.Push(res)

	return NONE, nil
}

// Abs pops an integer off of the stack and pushes its absolute value onto the stack.
// Returns an error if the popped value is not an integer or if the absolute value cannot be taken
func Abs(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	i, err := ctx.Estack.PopInt()
	if err != nil {
		return FAULT, err
	}

	a, err := i.Abs()
	if err != nil {
		return FAULT, err
	}

	ctx.Estack.Push(a)

	return NONE, nil
}

// Not flips the stack Item's value.
// If the value is True, it is flipped to False and viceversa.
func Not(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	b, err := ctx.Estack.PopBoolean()
	if err != nil {
		return FAULT, err
	}

	ctx.Estack.Push(b.Not())

	return NONE, nil
}

// Sign puts the sign of the top stack Item on top of the stack.
// If value is negative, put -1;
// If positive, put 1;
// If value is zero, put 0.
func Sign(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	i, err := ctx.Estack.PopInt()
	if err != nil {
		return FAULT, err
	}

	s := int64(i.Value().Sign())
	sign, err := stack.NewInt(big.NewInt(s))
	if err != nil {
		return FAULT, err
	}

	ctx.Estack.Push(sign)

	return NONE, nil
}

// Negate flips the sign of the stack Item.
func Negate(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	i, err := ctx.Estack.PopInt()
	if err != nil {
		return FAULT, err
	}

	a := big.NewInt(0).Neg(i.Value())
	b, err := stack.NewInt(a)
	if err != nil {
		return FAULT, err
	}

	ctx.Estack.Push(b)

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
