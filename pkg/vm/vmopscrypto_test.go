package vm

import (
	"encoding/hex"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/vm/stack"
	"github.com/stretchr/testify/assert"
)

func TestSha1Op(t *testing.T) {

	v := VM{}

	ba1 := stack.NewByteArray([]byte("this is test string"))

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(ba1)

	_, err := v.executeOp(stack.SHA1, ctx)
	assert.Nil(t, err)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.Pop()
	assert.Nil(t, err)

	ba2, err := item.ByteArray()
	assert.Nil(t, err)

	assert.Equal(t, "62d40fe74cf301cbfbe55c2679b96352449fb26d", hex.EncodeToString(ba2.Value()))
}

func TestSha256Op(t *testing.T) {

	v := VM{}

	ba1 := stack.NewByteArray([]byte("this is test string"))

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(ba1)

	_, err := v.executeOp(stack.SHA256, ctx)
	assert.Nil(t, err)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.Pop()
	assert.Nil(t, err)

	ba2, err := item.ByteArray()
	assert.Nil(t, err)

	assert.Equal(t, "8e76c5b9e6be2559bedccbd0ff104ebe02358ba463a44a68e96caf55f9400de5", hex.EncodeToString(ba2.Value()))
}

func TestHash160Op(t *testing.T) {

	v := VM{}

	ba1 := stack.NewByteArray([]byte("this is test string"))

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(ba1)

	_, err := v.executeOp(stack.HASH160, ctx)
	assert.Nil(t, err)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.Pop()
	assert.Nil(t, err)

	ba2, err := item.ByteArray()
	assert.Nil(t, err)

	assert.Equal(t, "e9c052b05a762ca9961a975db52e5417d99d958c", hex.EncodeToString(ba2.Value()))
}

func TestHash256Op(t *testing.T) {

	v := VM{}

	ba1 := stack.NewByteArray([]byte("this is test string"))

	ctx := stack.NewContext([]byte{})
	ctx.Estack.Push(ba1)

	_, err := v.executeOp(stack.HASH256, ctx)
	assert.Nil(t, err)

	// Stack should have one item
	assert.Equal(t, 1, ctx.Estack.Len())

	item, err := ctx.Estack.Pop()
	assert.Nil(t, err)

	ba2, err := item.ByteArray()
	assert.Nil(t, err)

	assert.Equal(t, "90ef790ee2557a3f9a1ba0e6910a9ff0ea75af3767ea7380760d729ac9927a60", hex.EncodeToString(ba2.Value()))
}
