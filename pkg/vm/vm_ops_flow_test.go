package vm

import (
	"math/big"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/vm/stack"
	"github.com/stretchr/testify/assert"
)

func TestNopOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(10))
	assert.Nil(t, err)

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(a)

	_, err = v.executeOp(stack.NOP, ctx)
	assert.Nil(t, err)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.PopInt()
	assert.Nil(t, err)

	assert.Equal(t, int64(10), item.Value().Int64())
}

func TestJmpOp(t *testing.T) {

	v := VM{}

	a, err := stack.NewInt(big.NewInt(10))
	assert.Nil(t, err)

	ctx := stack.NewContext([]byte{5, 0, 2, 3, 4})
	ctx.Estack.Push(a)

	// ctx.ip = -1
	// ctx.IP() = ctx.ip + 1
	assert.Equal(t, 0, ctx.IP())

	// ctx.ip will be set to offset.
	// offset = ctx.IP() + int(ctx.ReadInt16()) - 3
	//        = 0 + 5 -3 = 2
	_, err = v.executeOp(stack.JMP, ctx)
	assert.Nil(t, err)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	// ctx.IP() = ctx.ip + 1
	assert.Equal(t, 3, ctx.IP())
}

// test JMPIF instruction with true boolean
// on top of the stack
func TestJmpIfOp1(t *testing.T) {

	v := VM{}

	a := stack.NewBoolean(true)

	ctx := stack.NewContext([]byte{5, 0, 2, 3, 4})
	ctx.Estack.Push(a)

	// ctx.ip = -1
	// ctx.IP() = ctx.ip + 1
	assert.Equal(t, 0, ctx.IP())

	// ctx.ip will be set to offset
	// because the there is a true boolean
	// on top of the stack.
	// offset = ctx.IP() + int(ctx.ReadInt16()) - 3
	//        = 0 + 5 -3 = 2
	_, err := v.executeOp(stack.JMPIF, ctx)
	assert.Nil(t, err)

	// Stack should have 0 item
	assert.Equal(t, 0, ctx.Estack.Len())

	// ctx.IP() = ctx.ip + 1
	assert.Equal(t, 3, ctx.IP())
}

// test JMPIF instruction with false boolean
// on top of the stack
func TestJmpIfOp2(t *testing.T) {

	v := VM{}

	a := stack.NewBoolean(false)

	ctx := stack.NewContext([]byte{5, 0, 2, 3, 4})
	ctx.Estack.Push(a)

	// ctx.ip = -1
	// ctx.IP() = ctx.ip + 1
	assert.Equal(t, 0, ctx.IP())

	// nothing will happen because
	// the value of the boolean on top of the stack
	// is false
	_, err := v.executeOp(stack.JMPIF, ctx)
	assert.Nil(t, err)

	// Stack should have 0 item
	assert.Equal(t, 0, ctx.Estack.Len())

	// ctx.IP() = ctx.ip + 1
	assert.Equal(t, 0, ctx.IP())
}

// test JMPIFNOT instruction with true boolean
// on top of the stack
func TestJmpIfNotOp1(t *testing.T) {

	v := VM{}

	a := stack.NewBoolean(true)

	ctx := stack.NewContext([]byte{5, 0, 2, 3, 4})
	ctx.Estack.Push(a)

	// ctx.ip = -1
	// ctx.IP() = ctx.ip + 1
	assert.Equal(t, 0, ctx.IP())

	// nothing will happen because
	// the value of the boolean on top of the stack
	// is true
	_, err := v.executeOp(stack.JMPIFNOT, ctx)
	assert.Nil(t, err)

	// Stack should have 0 item
	assert.Equal(t, 0, ctx.Estack.Len())

	// ctx.IP() = ctx.ip + 1
	assert.Equal(t, 0, ctx.IP())
}

// test JMPIFNOT instruction with false boolean
// on top of the stack
func TestJmpIfNotOp2(t *testing.T) {

	v := VM{}

	a := stack.NewBoolean(false)

	ctx := stack.NewContext([]byte{5, 0, 2, 3, 4})
	ctx.Estack.Push(a)

	// ctx.ip = -1
	// ctx.IP() = ctx.ip + 1
	assert.Equal(t, 0, ctx.IP())

	// ctx.ip will be set to offset
	// because the there is a false boolean
	// on top of the stack.
	// offset = ctx.IP() + int(ctx.ReadInt16()) - 3
	//        = 0 + 5 -3 = 2
	_, err := v.executeOp(stack.JMPIFNOT, ctx)
	assert.Nil(t, err)

	// Stack should have one item
	assert.Equal(t, 0, ctx.Estack.Len())

	// ctx.IP() = ctx.ip + 1
	assert.Equal(t, 3, ctx.IP())
}
