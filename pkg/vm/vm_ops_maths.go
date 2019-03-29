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

// Div divides one stack Item by an other.
// Returns an error if either items cannot be casted to an integer
// or if the division of the integers cannot be performed.
func Div(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	operandA, operandB, err := popTwoIntegers(ctx)
	if err != nil {
		return FAULT, err
	}
	res, err := operandB.Div(operandA)
	if err != nil {
		return FAULT, err
	}

	ctx.Estack.Push(res)

	return NONE, nil
}

// Mod returns the mod of two stack Items.
// Returns an error if either items cannot be casted to an integer
// or if the mode of the integers cannot be performed.
func Mod(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	operandA, operandB, err := popTwoIntegers(ctx)
	if err != nil {
		return FAULT, err
	}
	res, err := operandB.Mod(operandA)
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

// BoolAnd pops two booleans off of the stack and pushes a boolean to the stack
// whose value is true iff both booleans' values are true.
// Returns an error if either items cannot be casted to an boolean
func BoolAnd(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	bool1, bool2, err := popTwoBooleans(ctx)
	if err != nil {
		return FAULT, err
	}
	res := bool1.And(bool2)
	if err != nil {
		return FAULT, err
	}

	ctx.Estack.Push(res)

	return NONE, nil
}

// BoolOr pops two booleans off of the stack and pushes a boolean to the stack
// whose value is true iff at least one of the two booleans' value is true.
// Returns an error if either items cannot be casted to an boolean
func BoolOr(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	bool1, bool2, err := popTwoBooleans(ctx)
	if err != nil {
		return FAULT, err
	}
	res := bool1.Or(bool2)
	if err != nil {
		return FAULT, err
	}

	ctx.Estack.Push(res)

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

// Shl pops two integers, a and b, off of the stack and pushes an integer to the stack
// whose value is the b's value shift to the left by a's value bits.
// Returns an error if either items cannot be casted to an integer
// or if the left shift operation cannot per performed with the two integer's value.
func Shl(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	a, b, err := popTwoIntegers(ctx)
	if err != nil {
		return FAULT, err
	}
	res, err := b.Lsh(a)
	if err != nil {
		return FAULT, err
	}

	ctx.Estack.Push(res)

	return NONE, nil
}

// Shr pops two integers, a and b, off of the stack and pushes an integer to the stack
// whose value is the b's value shift to the right by a's value bits.
// Returns an error if either items cannot be casted to an integer
// or if the right shift operation cannot per performed with the two integer's value.
func Shr(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	a, b, err := popTwoIntegers(ctx)
	if err != nil {
		return FAULT, err
	}
	res, err := b.Rsh(a)
	if err != nil {
		return FAULT, err
	}

	ctx.Estack.Push(res)

	return NONE, nil
}

// Lt pops two integers, a and b, off of the stack and pushes a boolean the stack
// whose value is true if a's value is less than b's value.
// Returns an error if either items cannot be casted to an integer
func Lt(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	operandA, operandB, err := popTwoIntegers(ctx)
	if err != nil {
		return FAULT, err
	}
	res := operandB.Lt(operandA)

	ctx.Estack.Push(stack.NewBoolean(res))

	return NONE, nil
}

// Gt pops two integers, a and b, off of the stack and pushes a boolean the stack
// whose value is true if a's value is greated than b's value.
// Returns an error if either items cannot be casted to an integer
func Gt(op stack.Instruction, ctx *stack.Context, istack *stack.Invocation, rstack *stack.RandomAccess) (Vmstate, error) {

	operandA, operandB, err := popTwoIntegers(ctx)
	if err != nil {
		return FAULT, err
	}
	res := operandB.Gt(operandA)

	ctx.Estack.Push(stack.NewBoolean(res))

	return NONE, nil
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

func popTwoBooleans(ctx *stack.Context) (*stack.Boolean, *stack.Boolean, error) {
	bool1, err := ctx.Estack.PopBoolean()
	if err != nil {
		return nil, nil, err
	}
	bool2, err := ctx.Estack.PopBoolean()
	if err != nil {
		return nil, nil, err
	}

	return bool1, bool2, nil
}
