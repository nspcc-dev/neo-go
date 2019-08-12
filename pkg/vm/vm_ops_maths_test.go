package vm

import (
	"math/big"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/vm/stack"
	"github.com/stretchr/testify/assert"
)

func TestIncOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(20))
	assert.Nil(t, err)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a)

	_, err = v.executeOp(stack.INC, ctx)
	assert.Nil(t, err)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	assert.Equal(t, int64(21), item.Value().Int64())
}

func TestDecOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(20))
	assert.Nil(t, err)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a)

	_, err = v.executeOp(stack.DEC, ctx)
	assert.Nil(t, err)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	assert.Equal(t, int64(19), item.Value().Int64())
}

func TestAddOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(20))
	assert.Nil(t, err)

	b, err := stack.NewInt(big.NewInt(23))
	assert.Nil(t, err)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a).Push(b)

	_, err = v.executeOp(stack.ADD, ctx)
	assert.Nil(t, err)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	assert.Equal(t, int64(43), item.Value().Int64())

}

func TestSubOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(30))
	assert.Nil(t, err)

	b, err := stack.NewInt(big.NewInt(40))
	assert.Nil(t, err)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a).Push(b)

	_, err = v.executeOp(stack.SUB, ctx)
	assert.Nil(t, err)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	assert.Equal(t, int64(-10), item.Value().Int64())

}

func TestDivOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(10))
	assert.Nil(t, err)

	b, err := stack.NewInt(big.NewInt(4))
	assert.Nil(t, err)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a).Push(b)

	_, err = v.executeOp(stack.DIV, ctx)
	assert.Nil(t, err)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	assert.Equal(t, int64(2), item.Value().Int64())
}

func TestModOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(15))
	assert.Nil(t, err)

	b, err := stack.NewInt(big.NewInt(4))
	assert.Nil(t, err)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a).Push(b)

	_, err = v.executeOp(stack.MOD, ctx)
	assert.Nil(t, err)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	assert.Equal(t, int64(3), item.Value().Int64())
}

func TestNzOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(20))
	assert.Nil(t, err)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a)

	_, err = v.executeOp(stack.NZ, ctx)
	assert.Nil(t, err)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopBoolean()
	assert.Nil(t, err)

	assert.Equal(t, true, item.Value())
}

func TestMulOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(20))
	assert.Nil(t, err)

	b, err := stack.NewInt(big.NewInt(20))
	assert.Nil(t, err)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a).Push(b)

	_, err = v.executeOp(stack.MUL, ctx)
	assert.Nil(t, err)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	assert.Equal(t, int64(400), item.Value().Int64())
}

func TestAbsOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(-20))
	assert.Nil(t, err)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a)

	_, err = v.executeOp(stack.ABS, ctx)
	assert.Nil(t, err)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	assert.Equal(t, int64(20), item.Value().Int64())
}

func TestNotOp(t *testing.T) {

	v := VM{}

	b := stack.NewBoolean(false)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(b)

	_, err := v.executeOp(stack.NOT, ctx)
	assert.Nil(t, err)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopBoolean()
	assert.Nil(t, err)

	assert.Equal(t, true, item.Value())
}

func TestNumEqual(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(6))
	assert.Nil(t, err)

	b, err := stack.NewInt(big.NewInt(6))
	assert.Nil(t, err)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a).Push(b)

	_, err = v.executeOp(stack.NUMEQUAL, ctx)
	assert.Nil(t, err)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopBoolean()
	assert.Nil(t, err)

	assert.Equal(t, true, item.Value())
}

func TestNumNotEqual(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(5))
	assert.Nil(t, err)

	b, err := stack.NewInt(big.NewInt(6))
	assert.Nil(t, err)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a).Push(b)

	_, err = v.executeOp(stack.NUMNOTEQUAL, ctx)
	assert.Nil(t, err)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopBoolean()
	assert.Nil(t, err)

	assert.Equal(t, true, item.Value())
}

func TestSignOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(-20))
	assert.Nil(t, err)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a)

	_, err = v.executeOp(stack.SIGN, ctx)
	assert.Nil(t, err)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	assert.Equal(t, int64(-1), item.Value().Int64())
}

func TestNegateOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(-20))
	assert.Nil(t, err)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a)

	_, err = v.executeOp(stack.NEGATE, ctx)
	assert.Nil(t, err)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	assert.Equal(t, int64(20), item.Value().Int64())
}

func TestLteOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(10))
	assert.Nil(t, err)

	b, err := stack.NewInt(big.NewInt(10))
	assert.Nil(t, err)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a).Push(b)

	// b is the first item pop.
	// a is the second item pop.
	// we perform a <= b and place
	// the result on top of the evaluation
	// stack
	_, err = v.executeOp(stack.LTE, ctx)
	assert.Nil(t, err)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopBoolean()
	assert.Nil(t, err)

	assert.Equal(t, true, item.Value())
}

func TestGteOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(10))
	assert.Nil(t, err)

	b, err := stack.NewInt(big.NewInt(2))
	assert.Nil(t, err)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a).Push(b)

	// b is the first item pop.
	// a is the second item pop.
	// we perform a >= b and place
	// the result on top of the evaluation
	// stack
	_, err = v.executeOp(stack.GTE, ctx)
	assert.Nil(t, err)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopBoolean()
	assert.Nil(t, err)

	assert.Equal(t, true, item.Value())
}

func TestShlOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(2))
	assert.Nil(t, err)

	b, err := stack.NewInt(big.NewInt(3))
	assert.Nil(t, err)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a).Push(b)

	// b is the first item pop.
	// a is the second item pop.
	// we perform a.Lsh(b) and place
	// the result on top of the evaluation
	// stack
	_, err = v.executeOp(stack.SHL, ctx)
	assert.Nil(t, err)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	assert.Equal(t, int64(16), item.Value().Int64())
}

func TestShrOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(10))
	assert.Nil(t, err)

	b, err := stack.NewInt(big.NewInt(2))
	assert.Nil(t, err)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a).Push(b)

	// b is the first item pop.
	// a is the second item pop.
	// we perform a.Rsh(b) and place
	// the result on top of the evaluation
	// stack
	_, err = v.executeOp(stack.SHR, ctx)
	assert.Nil(t, err)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	assert.Equal(t, int64(2), item.Value().Int64())
}

func TestBoolAndOp(t *testing.T) {

	v := VM{}

	a := stack.NewBoolean(true)
	b := stack.NewBoolean(true)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a).Push(b)

	_, err := v.executeOp(stack.BOOLAND, ctx)
	assert.Nil(t, err)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopBoolean()
	assert.Nil(t, err)

	assert.Equal(t, true, item.Value())
}

func TestBoolOrOp(t *testing.T) {

	v := VM{}

	a := stack.NewBoolean(false)
	b := stack.NewBoolean(true)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a).Push(b)

	_, err := v.executeOp(stack.BOOLOR, ctx)
	assert.Nil(t, err)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopBoolean()
	assert.Nil(t, err)

	assert.Equal(t, true, item.Value())
}

func TestLtOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(10))
	assert.Nil(t, err)

	b, err := stack.NewInt(big.NewInt(2))
	assert.Nil(t, err)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a).Push(b)

	// b is the first item pop.
	// a is the second item pop.
	// we perform a < b and place
	// the result on top of the evaluation
	// stack
	_, err = v.executeOp(stack.LT, ctx)
	assert.Nil(t, err)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopBoolean()
	assert.Nil(t, err)

	assert.Equal(t, false, item.Value())
}

func TestGtOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(10))
	assert.Nil(t, err)

	b, err := stack.NewInt(big.NewInt(2))
	assert.Nil(t, err)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a).Push(b)

	// b is the first item pop.
	// a is the second item pop.
	// we perform a > b and place
	// the result on top of the evaluation
	// stack
	_, err = v.executeOp(stack.GT, ctx)
	assert.Nil(t, err)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopBoolean()
	assert.Nil(t, err)

	assert.Equal(t, true, item.Value())
}

func TestMinOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(10))
	assert.Nil(t, err)

	b, err := stack.NewInt(big.NewInt(2))
	assert.Nil(t, err)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a).Push(b)

	_, err = v.executeOp(stack.MIN, ctx)
	assert.Nil(t, err)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	assert.Equal(t, int64(2), item.Value().Int64())
}

func TestMaxOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(10))
	assert.Nil(t, err)

	b, err := stack.NewInt(big.NewInt(2))
	assert.Nil(t, err)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a).Push(b)

	_, err = v.executeOp(stack.MAX, ctx)
	assert.Nil(t, err)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	assert.Equal(t, int64(10), item.Value().Int64())
}

func TestWithinOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(5))
	assert.Nil(t, err)

	b, err := stack.NewInt(big.NewInt(2))
	assert.Nil(t, err)

	c, err := stack.NewInt(big.NewInt(10))
	assert.Nil(t, err)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a).Push(b).Push(c)

	// c is the first item popped.
	// b is the second item popped.
	// a is the third item popped.
	// if a is within [b, c) we place a boolean,
	// whose value is true, on top of the evaluation
	// stack. Otherwise we place a boolean with
	// false value.
	_, err = v.executeOp(stack.WITHIN, ctx)
	assert.Nil(t, err)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopBoolean()
	assert.Nil(t, err)

	assert.Equal(t, true, item.Value())
}
