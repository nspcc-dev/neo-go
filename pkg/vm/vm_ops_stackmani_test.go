package vm

import (
	"math/big"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/vm/stack"
	"github.com/stretchr/testify/assert"
)

func TestRollOp(t *testing.T) {

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
	// and move the n-item which
	// has index len(stack)-n-1 (= 3-2-1= 0)
	// onto the top stack item.
	// The final stack will be [b,c,a]
	v.executeOp(stack.ROLL, ctx)

	// Stack should have three items
	assert.Equal(t, 3, ctx.Estack.Len())

	itemA, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	itemC, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	itemB, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	assert.Equal(t, int64(3), itemA.Value().Int64())
	assert.Equal(t, int64(9), itemC.Value().Int64())
	assert.Equal(t, int64(6), itemB.Value().Int64())
}

func TestRotOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(3))
	assert.Nil(t, err)

	b, err := stack.NewInt(big.NewInt(6))
	assert.Nil(t, err)

	c, err := stack.NewInt(big.NewInt(9))
	assert.Nil(t, err)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a).Push(b).Push(c)

	// move the third top stack a item  onto
	// the top stack item c.
	// The final stack will be [b,c,a]
	v.executeOp(stack.ROT, ctx)

	// Stack should have three items
	assert.Equal(t, 3, ctx.Estack.Len())

	itemA, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	itemC, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	itemB, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	assert.Equal(t, int64(3), itemA.Value().Int64())
	assert.Equal(t, int64(9), itemC.Value().Int64())
	assert.Equal(t, int64(6), itemB.Value().Int64())
}

func TestSwapOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(3))
	assert.Nil(t, err)

	b, err := stack.NewInt(big.NewInt(6))
	assert.Nil(t, err)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a).Push(b)

	// Swaps the top two stack items.
	// The final stack will be [b,a]
	v.executeOp(stack.SWAP, ctx)

	// Stack should have two items
	assert.Equal(t, 2, ctx.Estack.Len())

	itemA, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	itemB, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	assert.Equal(t, int64(3), itemA.Value().Int64())
	assert.Equal(t, int64(6), itemB.Value().Int64())

}

func TestTuckOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(3))
	assert.Nil(t, err)

	b, err := stack.NewInt(big.NewInt(6))
	assert.Nil(t, err)

	c, err := stack.NewInt(big.NewInt(9))
	assert.Nil(t, err)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a).Push(b).Push(c)

	// copy the top stack item c and
	// inserts it before the second top stack item.
	// The final stack will be [a,c,b,c]
	v.executeOp(stack.TUCK, ctx)

	// Stack should have four items
	assert.Equal(t, 4, ctx.Estack.Len())

	itemC, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	itemB, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	itemC2, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	itemA, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	assert.Equal(t, int64(9), itemC.Value().Int64())
	assert.Equal(t, int64(6), itemB.Value().Int64())
	assert.Equal(t, int64(9), itemC2.Value().Int64())
	assert.Equal(t, int64(3), itemA.Value().Int64())

}
