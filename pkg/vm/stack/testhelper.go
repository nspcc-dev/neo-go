package stack

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
)

// helper functions
func testPeakInteger(t *testing.T, tStack *RandomAccess, n uint16) *Int {
	stackElement, err := tStack.Peek(n)
	assert.Nil(t, err)
	item, err := stackElement.Integer()
	if err != nil {
		t.Fail()
	}
	return item
}

func testPopInteger(t *testing.T, tStack *RandomAccess) *Int {
	stackElement, err := tStack.Pop()
	assert.Nil(t, err)
	item, err := stackElement.Integer()
	if err != nil {
		t.Fail()
	}
	return item
}

func testMakeStackInt(t *testing.T, num int64) *Int {
	a, err := NewInt(big.NewInt(num))
	assert.Nil(t, err)
	return a
}
