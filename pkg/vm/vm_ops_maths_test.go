package vm

import (
	"math/big"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/vm/stack"
	"github.com/stretchr/testify/assert"
)

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

	v.ExecuteOp(stack.ADD, ctx)

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

	v.ExecuteOp(stack.SUB, ctx)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopInt()
	if err != nil {
		t.Fail()
	}

	assert.Equal(t, int64(-10), item.Value().Int64())

}
