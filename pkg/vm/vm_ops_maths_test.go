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
	if err != nil {
		t.Fail()
	}

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a)

	v.executeOp(stack.INC, ctx)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopInt()
	if err != nil {
		t.Fail()
	}

	assert.Equal(t, int64(21), item.Value().Int64())
}

func TestDecOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(20))
	if err != nil {
		t.Fail()
	}

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a)

	v.executeOp(stack.DEC, ctx)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopInt()
	if err != nil {
		t.Fail()
	}

	assert.Equal(t, int64(19), item.Value().Int64())
}

func TestAddOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(20))
	if err != nil {
		t.Fail()
	}
	b, err := stack.NewInt(big.NewInt(23))
	if err != nil {
		t.Fail()
	}

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a).Push(b)

	v.executeOp(stack.ADD, ctx)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopInt()
	if err != nil {
		t.Fail()
	}

	assert.Equal(t, int64(43), item.Value().Int64())

}

func TestSubOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(30))
	if err != nil {
		t.Fail()
	}
	b, err := stack.NewInt(big.NewInt(40))
	if err != nil {
		t.Fail()
	}

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a).Push(b)

	v.executeOp(stack.SUB, ctx)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopInt()
	if err != nil {
		t.Fail()
	}

	assert.Equal(t, int64(-10), item.Value().Int64())

}

func TestDivOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(10))
	if err != nil {
		t.Fail()
	}
	b, err := stack.NewInt(big.NewInt(4))
	if err != nil {
		t.Fail()
	}

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a).Push(b)

	v.executeOp(stack.DIV, ctx)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopInt()
	if err != nil {
		t.Fail()
	}

	assert.Equal(t, int64(2), item.Value().Int64())
}

func TestModOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(15))
	if err != nil {
		t.Fail()
	}
	b, err := stack.NewInt(big.NewInt(4))
	if err != nil {
		t.Fail()
	}

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a).Push(b)

	v.executeOp(stack.MOD, ctx)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopInt()
	if err != nil {
		t.Fail()
	}

	assert.Equal(t, int64(3), item.Value().Int64())
}

func TestNzOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(20))
	if err != nil {
		t.Fail()
	}

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a)

	v.executeOp(stack.NZ, ctx)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopBoolean()
	if err != nil {
		t.Fail()
	}

	assert.Equal(t, true, item.Value())
}

func TestMulOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(20))
	if err != nil {
		t.Fail()
	}
	b, err := stack.NewInt(big.NewInt(20))
	if err != nil {
		t.Fail()
	}

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a).Push(b)

	v.executeOp(stack.MUL, ctx)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopInt()
	if err != nil {
		t.Fail()
	}

	assert.Equal(t, int64(400), item.Value().Int64())
}

func TestAbsOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(-20))
	if err != nil {
		t.Fail()
	}

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a)

	v.executeOp(stack.ABS, ctx)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopInt()
	if err != nil {
		t.Fail()
	}

	assert.Equal(t, int64(20), item.Value().Int64())
}

func TestNotOp(t *testing.T) {

	v := VM{}

	b := stack.NewBoolean(false)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(b)

	v.executeOp(stack.NOT, ctx)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopBoolean()
	if err != nil {
		t.Fail()
	}

	assert.Equal(t, true, item.Value())
}

func TestSignOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(-20))
	if err != nil {
		t.Fail()
	}

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a)

	v.executeOp(stack.SIGN, ctx)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopInt()
	if err != nil {
		t.Fail()
	}

	assert.Equal(t, int64(-1), item.Value().Int64())
}

func TestNegateOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(-20))
	if err != nil {
		t.Fail()
	}

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a)

	v.executeOp(stack.NEGATE, ctx)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopInt()
	if err != nil {
		t.Fail()
	}

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
	v.executeOp(stack.LTE, ctx)

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
	v.executeOp(stack.GTE, ctx)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopBoolean()
	assert.Nil(t, err)

	assert.Equal(t, true, item.Value())
}

func TestShlOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(2))
	if err != nil {
		t.Fail()
	}
	b, err := stack.NewInt(big.NewInt(3))
	if err != nil {
		t.Fail()
	}

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a).Push(b)

	// b is the first item pop.
	// a is the second item pop.
	// we perform a.Lsh(b) and place
	// the result on top of the evaluation
	// stack
	v.executeOp(stack.SHL, ctx)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopInt()
	if err != nil {
		t.Fail()
	}

	assert.Equal(t, int64(16), item.Value().Int64())
}

func TestShrOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(10))
	if err != nil {
		t.Fail()
	}
	b, err := stack.NewInt(big.NewInt(2))
	if err != nil {
		t.Fail()
	}

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a).Push(b)

	// b is the first item pop.
	// a is the second item pop.
	// we perform a.Rsh(b) and place
	// the result on top of the evaluation
	// stack
	v.executeOp(stack.SHR, ctx)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopInt()
	if err != nil {
		t.Fail()
	}

	assert.Equal(t, int64(2), item.Value().Int64())
}

func TestBoolAndOp(t *testing.T) {

	v := VM{}

	a := stack.NewBoolean(true)
	b := stack.NewBoolean(true)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a).Push(b)

	v.executeOp(stack.BOOLAND, ctx)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopBoolean()
	if err != nil {
		t.Fail()
	}

	assert.Equal(t, true, item.Value())
}

func TestBoolOrOp(t *testing.T) {

	v := VM{}

	a := stack.NewBoolean(false)
	b := stack.NewBoolean(true)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a).Push(b)

	v.executeOp(stack.BOOLOR, ctx)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopBoolean()
	if err != nil {
		t.Fail()
	}

	assert.Equal(t, true, item.Value())
}

func TestLtOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(10))
	assert.NoError(t, err)

	b, err := stack.NewInt(big.NewInt(2))
	assert.NoError(t, err)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a).Push(b)

	// b is the first item pop.
	// a is the second item pop.
	// we perform a < b and place
	// the result on top of the evaluation
	// stack
	v.executeOp(stack.LT, ctx)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopBoolean()
	assert.NoError(t, err)

	assert.Equal(t, false, item.Value())
}

func TestGtOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(10))
	assert.NoError(t, err)

	b, err := stack.NewInt(big.NewInt(2))
	assert.NoError(t, err)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a).Push(b)

	// b is the first item pop.
	// a is the second item pop.
	// we perform a > b and place
	// the result on top of the evaluation
	// stack
	v.executeOp(stack.GT, ctx)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopBoolean()
	assert.NoError(t, err)

	assert.Equal(t, true, item.Value())
}
