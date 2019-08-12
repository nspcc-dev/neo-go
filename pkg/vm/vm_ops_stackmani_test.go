package vm

import (
	"math/big"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/vm/stack"
	"github.com/stretchr/testify/assert"
)

func TestDupFromAltStackOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(10))
	assert.Nil(t, err)

	b, err := stack.NewInt(big.NewInt(2))
	assert.Nil(t, err)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a)
	ctx.Astack.Push(b)

	v.executeOp(stack.DUPFROMALTSTACK, ctx)

	assert.Equal(t, 1, ctx.Astack.Len())
	assert.Equal(t, 2, ctx.Estack.Len())

	itemE, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	itemA, err := ctx.Astack.PopInt()
	assert.Nil(t, err)

	assert.Equal(t, int64(2), itemE.Value().Int64())
	assert.Equal(t, int64(2), itemA.Value().Int64())
}

func TestToAltStackOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(10))
	assert.Nil(t, err)

	b, err := stack.NewInt(big.NewInt(2))
	assert.Nil(t, err)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a)
	ctx.Astack.Push(b)

	v.executeOp(stack.TOALTSTACK, ctx)

	assert.Equal(t, 2, ctx.Astack.Len())
	assert.Equal(t, 0, ctx.Estack.Len())

	item, err := ctx.Astack.PopInt()
	assert.Nil(t, err)

	assert.Equal(t, int64(10), item.Value().Int64())
}

func TestFromAltStackOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(10))
	assert.Nil(t, err)

	b, err := stack.NewInt(big.NewInt(2))
	assert.Nil(t, err)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a)
	ctx.Astack.Push(b)

	v.executeOp(stack.FROMALTSTACK, ctx)

	assert.Equal(t, 0, ctx.Astack.Len())
	assert.Equal(t, 2, ctx.Estack.Len())

	item, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	assert.Equal(t, int64(2), item.Value().Int64())
}

func TestXDropOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(3))
	assert.Nil(t, err)

	b, err := stack.NewInt(big.NewInt(6))
	assert.Nil(t, err)

	c, err := stack.NewInt(big.NewInt(9))
	assert.Nil(t, err)

	d, err := stack.NewInt(big.NewInt(2))
	assert.Nil(t, err)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a)
	ctx.Estack.Push(b)
	ctx.Estack.Push(c)
	ctx.Estack.Push(d)

	// pop n (= d = 2) from the stack.
	// we will remove the n-item which
	// is located at position
	// len(stack)-n-1 = 3-2-1 = 0.
	// Therefore a is removed from the stack.
	// Only b, c remain on the stack.
	v.executeOp(stack.XDROP, ctx)

	assert.Equal(t, 2, ctx.Estack.Len())

	itemC, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	itemB, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	assert.Equal(t, int64(6), itemB.Value().Int64())
	assert.Equal(t, int64(9), itemC.Value().Int64())

}
