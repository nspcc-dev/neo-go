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
	_, err = v.executeOp(stack.ROLL, ctx)
	assert.Nil(t, err)

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
	_, err = v.executeOp(stack.ROT, ctx)
	assert.Nil(t, err)

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
	_, err = v.executeOp(stack.SWAP, ctx)
	assert.Nil(t, err)

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
	_, err = v.executeOp(stack.TUCK, ctx)
	assert.Nil(t, err)

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

func TestDupOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(3))
	assert.Nil(t, err)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a)

	_, err = v.executeOp(stack.DUP, ctx)
	assert.Nil(t, err)

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

	_, err = v.executeOp(stack.NIP, ctx)
	assert.Nil(t, err)

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
	_, err = v.executeOp(stack.OVER, ctx)
	assert.Nil(t, err)

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
	_, err = v.executeOp(stack.PICK, ctx)
	assert.Nil(t, err)

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
	_, err = v.executeOp(stack.XSWAP, ctx)
	assert.Nil(t, err)

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
	_, err = v.executeOp(stack.XTUCK, ctx)
	assert.Nil(t, err)

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
	_, err = v.executeOp(stack.DEPTH, ctx)
	assert.Nil(t, err)

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

func TestDupFromAltStackOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(10))
	assert.Nil(t, err)

	b, err := stack.NewInt(big.NewInt(2))
	assert.Nil(t, err)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a)
	ctx.Astack.Push(b)

	_, err = v.executeOp(stack.DUPFROMALTSTACK, ctx)
	assert.Nil(t, err)

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

	_, err = v.executeOp(stack.TOALTSTACK, ctx)
	assert.Nil(t, err)

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

	_, err = v.executeOp(stack.FROMALTSTACK, ctx)
	assert.Nil(t, err)

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
	_, err = v.executeOp(stack.XDROP, ctx)
	assert.Nil(t, err)

	assert.Equal(t, 2, ctx.Estack.Len())

	itemC, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	itemB, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	assert.Equal(t, int64(6), itemB.Value().Int64())
	assert.Equal(t, int64(9), itemC.Value().Int64())

}
