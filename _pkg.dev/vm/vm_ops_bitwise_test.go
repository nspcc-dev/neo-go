package vm

import (
	"math/big"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/vm/stack"
	"github.com/stretchr/testify/assert"
)

func TestInvertOp(t *testing.T) {

	v := VM{}

	// 0000 00110 = 5
	a, err := stack.NewInt(big.NewInt(5))
	assert.Nil(t, err)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a)

	// 1111 11001 = -6 (two complement representation)
	_, err = v.executeOp(stack.INVERT, ctx)
	assert.Nil(t, err)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	assert.Equal(t, int64(-6), item.Value().Int64())
}

func TestAndOp(t *testing.T) {

	v := VM{}

	// 110001 = 49
	a, err := stack.NewInt(big.NewInt(49))
	assert.Nil(t, err)

	// 100011 = 35
	b, err := stack.NewInt(big.NewInt(35))
	assert.Nil(t, err)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a).Push(b)

	// 100001 = 33
	_, err = v.executeOp(stack.AND, ctx)
	assert.Nil(t, err)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	assert.Equal(t, int64(33), item.Value().Int64())
}

func TestOrOp(t *testing.T) {

	v := VM{}

	// 110001 = 49
	a, err := stack.NewInt(big.NewInt(49))
	assert.Nil(t, err)

	// 100011 = 35
	b, err := stack.NewInt(big.NewInt(35))
	assert.Nil(t, err)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a).Push(b)

	// 110011 = 51 (49 OR 35)
	_, err = v.executeOp(stack.OR, ctx)
	assert.Nil(t, err)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	assert.Equal(t, int64(51), item.Value().Int64())
}

func TestXorOp(t *testing.T) {

	v := VM{}

	// 110001 = 49
	a, err := stack.NewInt(big.NewInt(49))
	assert.Nil(t, err)

	// 100011 = 35
	b, err := stack.NewInt(big.NewInt(35))
	assert.Nil(t, err)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a).Push(b)

	// 010010 = 18 (49 XOR 35)
	_, err = v.executeOp(stack.XOR, ctx)
	assert.Nil(t, err)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	assert.Equal(t, int64(18), item.Value().Int64())
}

func TestEqualOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(10))
	assert.Nil(t, err)

	b, err := stack.NewInt(big.NewInt(10))
	assert.Nil(t, err)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a).Push(b)

	_, err = v.executeOp(stack.EQUAL, ctx)
	assert.Nil(t, err)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopBoolean()
	assert.Nil(t, err)

	assert.Equal(t, true, item.Value())
}
