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

func TestNumEqual(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(6))
	if err != nil {
		t.Fail()
	}
	b, err := stack.NewInt(big.NewInt(6))
	if err != nil {
		t.Fail()
	}

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a).Push(b)

	v.executeOp(stack.NUMEQUAL, ctx)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopBoolean()
	if err != nil {
		t.Fail()
	}

	assert.Equal(t, true, item.Value())
}

func TestNumNotEqual(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(5))
	if err != nil {
		t.Fail()
	}
	b, err := stack.NewInt(big.NewInt(6))
	if err != nil {
		t.Fail()
	}

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a).Push(b)

	v.executeOp(stack.NUMNOTEQUAL, ctx)

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
