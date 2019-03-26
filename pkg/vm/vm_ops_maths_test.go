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
