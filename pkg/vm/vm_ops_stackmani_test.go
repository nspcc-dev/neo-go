package vm

import (
	"math/big"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/vm/stack"
	"github.com/stretchr/testify/assert"
)

func TestXswapOp(t *testing.T) {

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
	ctx.Estack.Push(a).Push(b).Push(c).Push(d)

	// pop n (= d = 2) from the stack.
	// we will swap the n-item which
	// is located in position len(stack)-n-1 (= 3-2-1= 0)
	// with the top stack item.
	// The final stack will be [c,b,a]
	v.executeOp(stack.XSWAP, ctx)

	// Stack should have three items
	assert.Equal(t, 3, ctx.Estack.Len())

	itemA, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	itemB, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	itemC, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	assert.Equal(t, int64(3), itemA.Value().Int64())
	assert.Equal(t, int64(6), itemB.Value().Int64())
	assert.Equal(t, int64(9), itemC.Value().Int64())

}

func TestXTuckOp(t *testing.T) {

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
	ctx.Estack.Push(a).Push(b).Push(c).Push(d)

	// pop n (= d = 2) from the stack
	// and insert the top stack item c
	// to the position len(stack)-n (= 3-2 = 1)
	// of the stack.The final stack will be [a,c,b,c]
	v.executeOp(stack.XTUCK, ctx)

	// Stack should have four items
	assert.Equal(t, 4, ctx.Estack.Len())

	// c
	item0, err := ctx.Estack.PopInt()
	assert.Nil(t, err)
	// b
	item1, err := ctx.Estack.PopInt()
	assert.Nil(t, err)
	// c
	item2, err := ctx.Estack.PopInt()
	assert.Nil(t, err)
	// a
	item3, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	assert.Equal(t, int64(9), item0.Value().Int64())
	assert.Equal(t, int64(6), item1.Value().Int64())
	assert.Equal(t, int64(9), item2.Value().Int64())
	assert.Equal(t, int64(3), item3.Value().Int64())

}

func TestXDepthOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(3))
	assert.Nil(t, err)

	b, err := stack.NewInt(big.NewInt(6))
	assert.Nil(t, err)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a).Push(b)

	// push integer whose value is len(stack) (2)
	// on top of the stack
	v.executeOp(stack.DEPTH, ctx)

	// Stack should have three items
	assert.Equal(t, 3, ctx.Estack.Len())

	// len(stack)
	item0, err := ctx.Estack.PopInt()
	assert.Nil(t, err)
	// b
	item1, err := ctx.Estack.PopInt()
	assert.Nil(t, err)
	// a
	item2, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	assert.Equal(t, int64(2), item0.Value().Int64())
	assert.Equal(t, int64(6), item1.Value().Int64())
	assert.Equal(t, int64(3), item2.Value().Int64())
}

func TestXDropOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(3))
	assert.Nil(t, err)

	b, err := stack.NewInt(big.NewInt(6))
	assert.Nil(t, err)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a).Push(b)

	// Remove the top stack item from the stack.
	// The remaining stack is [a]
	v.executeOp(stack.DROP, ctx)

	// Stack should have 2 items
	assert.Equal(t, 1, ctx.Estack.Len())

	itemA, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	assert.Equal(t, int64(3), itemA.Value().Int64())
}
