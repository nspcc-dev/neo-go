package stack

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStackPushPop(t *testing.T) {
	// Create two stack Integers
	a, err := NewInt(big.NewInt(10))
	if err != nil {
		t.Fail()
	}
	b, err := NewInt(big.NewInt(20))
	if err != nil {
		t.Fail()
	}

	// Create a new stack
	testStack := New()

	// Push to stack
	testStack.Push(a).Push(b)

	// There should only be two values on the stack
	assert.Equal(t, 2, testStack.Len())

	// Pop first element and it should be equal to b
	stackElement, err := testStack.Pop()
	if err != nil {
		t.Fail()
	}
	item, err := stackElement.Integer()
	if err != nil {
		t.Fail()
	}
	assert.Equal(t, true, item.Equal(b))

	// Pop second element and it should be equal to a
	stackElement, err = testStack.Pop()
	if err != nil {
		t.Fail()
	}
	item, err = stackElement.Integer()
	if err != nil {
		t.Fail()
	}
	assert.Equal(t, true, item.Equal(a))

	// We should get an error as there are nomore items left to pop
	stackElement, err = testStack.Pop()
	assert.NotNil(t, err)

}

// For this test to pass, we should get an error when popping from a nil stack
// and we should initialise and push an element if pushing to an empty stack
func TestPushPopNil(t *testing.T) {

	// stack is nil when initialised without New constructor
	testStack := RandomAccess{}

	// Popping from nil stack
	// - should give an error
	// - element returned should be nil
	stackElement, err := testStack.Pop()
	assert.NotNil(t, err)
	assert.Nil(t, stackElement)

	// stack should still be nil after failing to pop
	assert.Nil(t, testStack.vals)

	// create a random test stack item
	a, err := NewInt(big.NewInt(2))
	assert.Nil(t, err)

	// push random item to stack
	testStack.Push(a)

	// push should initialise the stack and put one element on the stack
	assert.Equal(t, 1, testStack.Len())
}

// Test passes if we can peek and modify an item
//without modifying the value on the stack
func TestStackPeekMutability(t *testing.T) {

	testStack := New()

	a, err := NewInt(big.NewInt(2))
	assert.Nil(t, err)
	b, err := NewInt(big.NewInt(3))
	assert.Nil(t, err)

	testStack.Push(a).Push(b)

	peekedItem := testPeakInteger(t, testStack, 0)
	assert.Equal(t, true, peekedItem.Equal(b))

	// Check that by modifying the peeked value,
	// we did not modify the item on the stack
	peekedItem = a
	peekedItem.val = big.NewInt(0)

	// Pop item from stack and check it is still the same
	poppedItem := testPopInteger(t, testStack)
	assert.Equal(t, true, poppedItem.Equal(b))
}
func TestStackPeek(t *testing.T) {

	testStack := New()

	values := []int64{23, 45, 67, 89, 12, 344}
	for _, val := range values {
		a := testMakeStackInt(t, val)
		testStack.Push(a)
	}

	// i starts at 0, j starts at len(values)-1
	for i, j := 0, len(values)-1; j >= 0; i, j = i+1, j-1 {

		peekedItem := testPeakInteger(t, testStack, uint16(i))
		a := testMakeStackInt(t, values[j])

		fmt.Printf("%#v\n", peekedItem.val.Int64())

		assert.Equal(t, true, a.Equal(peekedItem))

	}

}

func TestStackInsert(t *testing.T) {

	testStack := New()

	a := testMakeStackInt(t, 2)
	b := testMakeStackInt(t, 4)
	c := testMakeStackInt(t, 6)

	// insert on an empty stack should put element on top
	_, err := testStack.Insert(0, a)
	assert.Equal(t, err, nil)
	_, err = testStack.Insert(0, b)
	assert.Equal(t, err, nil)
	_, err = testStack.Insert(1, c)
	assert.Equal(t, err, nil)

	// Order should be [a,c,b]
	pop1 := testPopInteger(t, testStack)
	pop2 := testPopInteger(t, testStack)
	pop3 := testPopInteger(t, testStack)

	assert.Equal(t, true, pop1.Equal(b))
	assert.Equal(t, true, pop2.Equal(c))
	assert.Equal(t, true, pop3.Equal(a))

}
