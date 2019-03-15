package stack

import (
	"errors"
	"fmt"
)

const (
	// StackAverageSize is used to set the capacity of the stack
	// setting this number too low, will cause extra allocations
	StackAverageSize = 20
)

// RandomAccess represents a Random Access Stack
type RandomAccess struct {
	vals []Item
}

// New will return a new random access stack
func New() *RandomAccess {
	return &RandomAccess{
		vals: make([]Item, 0, StackAverageSize),
	}
}

// Items will return all items in the stack
func (ras *RandomAccess) items() []Item {
	return ras.vals
}

//Len will return the length of the stack
func (ras *RandomAccess) Len() int {
	if ras.vals == nil {
		return -1
	}
	return len(ras.vals)
}

// Clear will remove all items in the stack
func (ras *RandomAccess) Clear() {
	ras.vals = make([]Item, 0, StackAverageSize)
}

// Pop will remove the last stack item that was added
func (ras *RandomAccess) Pop() (Item, error) {
	if len(ras.vals) == 0 {
		return nil, errors.New("There are no items on the stack to pop")
	}
	if ras.vals == nil {
		return nil, errors.New("Cannot pop from a nil stack")
	}

	l := len(ras.vals)
	item := ras.vals[l-1]
	ras.vals = ras.vals[:l-1]

	return item, nil
}

// Push will put a stack item onto the top of the stack
func (ras *RandomAccess) Push(item Item) *RandomAccess {
	if ras.vals == nil {
		ras.vals = make([]Item, 0, StackAverageSize)
	}

	ras.vals = append(ras.vals, item)

	return ras
}

// Insert will push a stackItem onto the stack at position `n`
// Note; index 0 is the top of the stack, which is the end of slice
func (ras *RandomAccess) Insert(n uint16, item Item) (*RandomAccess, error) {

	if n == 0 {
		return ras.Push(item), nil
	}

	if ras.vals == nil {
		ras.vals = make([]Item, 0, StackAverageSize)
	}

	// Check that we are not inserting out of the bounds
	stackSize := uint16(len(ras.vals))
	if n > stackSize-1 {
		return nil, fmt.Errorf("Tried to insert at index %d when length of stack is %d", n, len(ras.vals))
	}

	index := stackSize - n

	ras.vals = append(ras.vals, item)
	copy(ras.vals[index:], ras.vals[index-1:])
	ras.vals[index] = item

	return ras, nil
}

// Peek will check an element at a given index
// Note: 0 is the top of the stack, which is the end of the slice
func (ras *RandomAccess) Peek(n uint16) (Item, error) {

	stackSize := uint16(len(ras.vals))

	if n == 0 {
		index := stackSize - 1
		return ras.vals[index], nil
	}

	if ras.vals == nil {
		return nil, errors.New("Cannot peak at a nil stack")
	}

	// Check that we are not peeking out of the bounds
	if n > stackSize-1 {
		return nil, fmt.Errorf("Tried to peek at index %d when length of stack is %d", n, len(ras.vals))
	}

	index := stackSize - n - 1

	return ras.vals[index], nil
}

// Convenience Functions

// PopInt will remove the last stack item that was added
// And cast it to an integer
func (ras *RandomAccess) PopInt() (*Int, error) {
	item, err := ras.Pop()
	if err != nil {
		return nil, err
	}
	return item.Integer()
}
