package vm

import (
	"math/big"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/vm/stack"
	"github.com/stretchr/testify/assert"
)

func TestDupOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(3))
	assert.Nil(t, err)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a)

	v.executeOp(stack.DUP, ctx)

	// Stack should have two items
	assert.Equal(t, 2, ctx.Estack.Len())

	item1, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	item2, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	assert.Equal(t, int64(3), item1.Value().Int64())
	assert.Equal(t, int64(3), item2.Value().Int64())

}

func TestNipOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(3))
	assert.Nil(t, err)

	b, err := stack.NewInt(big.NewInt(6))
	assert.Nil(t, err)

	c, err := stack.NewInt(big.NewInt(9))
	assert.Nil(t, err)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a).Push(b).Push(c)

	v.executeOp(stack.NIP, ctx)

	// Stack should have two items
	assert.Equal(t, 2, ctx.Estack.Len())

	itemC, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	itemA, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	assert.Equal(t, int64(3), itemA.Value().Int64())
	assert.Equal(t, int64(9), itemC.Value().Int64())

}

func TestOverOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(3))
	assert.Nil(t, err)

	b, err := stack.NewInt(big.NewInt(6))
	assert.Nil(t, err)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a).Push(b)

	// OVER copies the second top stack item a
	// onto the top stack item b.
	// the new stack will be [a,b,a].
	v.executeOp(stack.OVER, ctx)

	// Stack should have three items
	assert.Equal(t, 3, ctx.Estack.Len())

	itemA, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	itemB, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	itemA2, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	assert.Equal(t, int64(3), itemA.Value().Int64())
	assert.Equal(t, int64(6), itemB.Value().Int64())
	assert.Equal(t, int64(3), itemA2.Value().Int64())

}

func TestPickOp(t *testing.T) {

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
	// we will copy the n-item which
	// has index len(stack)-n-1 (= 3-2-1= 0)
	// onto the top stack item.
	// The final stack will be [a,b,c,a]
	v.executeOp(stack.PICK, ctx)

	// Stack should have four items
	assert.Equal(t, 4, ctx.Estack.Len())

	itemA, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	itemC, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	itemB, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	itemA2, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	assert.Equal(t, int64(3), itemA.Value().Int64())
	assert.Equal(t, int64(9), itemC.Value().Int64())
	assert.Equal(t, int64(6), itemB.Value().Int64())
	assert.Equal(t, int64(3), itemA2.Value().Int64())

}

/*
func TestXswapOp(t *testing.T) {

	v := VM{}

	sert.Nil(t, err)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a).Push(b).Push(c).Push(d)

	// pop n (= d = 2) from the stack.
	// we wa, err := stack.NewInt(big.NewInt(3))
	assert.Nil(t, err)

	b, err := stack.NewInt(big.NewInt(6))
	assert.Nil(t, err)

	c, err := stack.NewInt(big.NewInt(9))
	assert.Nil(t, err)

	d, err := stack.NewInt(big.NewInt(2))
	asill swap the n-item which
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
*/
