package stack

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAdd(t *testing.T) {
	a := testMakeStackInt(t, 10)
	b := testMakeStackInt(t, 20)
	expected := testMakeStackInt(t, 30)
	c, err := a.Add(b)
	assert.Nil(t, err)
	assert.Equal(t, true, expected.Equal(c))
}
func TestSub(t *testing.T) {
	a := testMakeStackInt(t, 30)
	b := testMakeStackInt(t, 200)
	expected := testMakeStackInt(t, 170)
	c, err := b.Sub(a)
	assert.Nil(t, err)
	assert.Equal(t, true, expected.Equal(c))
}
func TestMul(t *testing.T) {
	a := testMakeStackInt(t, 10)
	b := testMakeStackInt(t, 20)
	expected := testMakeStackInt(t, 200)
	c, err := a.Mul(b)
	assert.Nil(t, err)
	assert.Equal(t, true, expected.Equal(c))
}
func TestMod(t *testing.T) {
	a := testMakeStackInt(t, 10)
	b := testMakeStackInt(t, 20)
	expected := testMakeStackInt(t, 10)
	c, err := a.Mod(b)
	assert.Nil(t, err)
	assert.Equal(t, true, expected.Equal(c))
}
func TestLsh(t *testing.T) {
	a := testMakeStackInt(t, 23)
	b := testMakeStackInt(t, 8)
	expected := testMakeStackInt(t, 5888)
	c, err := a.Lsh(b)
	assert.Nil(t, err)
	assert.Equal(t, true, expected.Equal(c))
}

func TestRsh(t *testing.T) {
	a := testMakeStackInt(t, 128)
	b := testMakeStackInt(t, 3)
	expected := testMakeStackInt(t, 16)
	c, err := a.Rsh(b)
	assert.Nil(t, err)
	assert.Equal(t, true, expected.Equal(c))
}

func TestByteArrConversion(t *testing.T) {

	var num int64 = 100000

	a := testMakeStackInt(t, num)
	ba, err := a.ByteArray()
	assert.Nil(t, err)

	assert.Equal(t, num, testReadInt64(t, ba.val))

	have, err := ba.Integer()
	assert.Nil(t, err)

	assert.Equal(t, num, have.val.Int64())

}
