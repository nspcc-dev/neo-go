package vm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPushElement(t *testing.T) {
	elems := makeElements(10)
	s := NewStack("test")
	for _, elem := range elems {
		s.Push(elem)
	}

	assert.Equal(t, len(elems), s.Len())

	for i := 0; i < len(elems); i++ {
		assert.Equal(t, elems[len(elems)-1-i], s.Peek(i))
	}
}

func TestPopElement(t *testing.T) {
	var (
		s     = NewStack("test")
		elems = makeElements(10)
	)
	for _, elem := range elems {
		s.Push(elem)
	}

	for i := len(elems) - 1; i >= 0; i-- {
		assert.Equal(t, elems[i], s.Pop())
		assert.Equal(t, i, s.Len())
	}
}

func TestPeekElement(t *testing.T) {
	var (
		s     = NewStack("test")
		elems = makeElements(10)
	)
	for _, elem := range elems {
		s.Push(elem)
	}
	for i := len(elems) - 1; i >= 0; i-- {
		assert.Equal(t, elems[i], s.Peek(len(elems)-i-1))
	}
}

func TestRemoveAt(t *testing.T) {
	var (
		s     = NewStack("test")
		elems = makeElements(10)
	)
	for _, elem := range elems {
		s.Push(elem)
	}

	elem := s.RemoveAt(8)
	assert.Equal(t, elems[1], elem)
	assert.Nil(t, elem.prev)
	assert.Nil(t, elem.next)
	assert.Nil(t, elem.stack)

	// Test if the pointers are moved.
	assert.Equal(t, elems[0], s.Peek(8))
	assert.Equal(t, elems[2], s.Peek(7))
}

func TestPushFromOtherStack(t *testing.T) {
	var (
		s1    = NewStack("test")
		s2    = NewStack("test2")
		elems = makeElements(2)
	)
	for _, elem := range elems {
		s1.Push(elem)
	}
	s2.Push(NewElement(100))
	s2.Push(NewElement(101))

	s1.Push(s2.Pop())
	assert.Equal(t, len(elems)+1, s1.Len())
	assert.Equal(t, 1, s2.Len())
}

func TestDupElement(t *testing.T) {
	s := NewStack("test")
	elemA := NewElement(101)
	s.Push(elemA)

	dupped := s.Dup(0)
	s.Push(dupped)
	assert.Equal(t, 2, s.Len())
	assert.Equal(t, dupped, s.Peek(0))
}

func TestBack(t *testing.T) {
	var (
		s     = NewStack("test")
		elems = makeElements(10)
	)
	for _, elem := range elems {
		s.Push(elem)
	}

	assert.Equal(t, elems[0], s.Back())
}

func TestTop(t *testing.T) {
	var (
		s     = NewStack("test")
		elems = makeElements(10)
	)
	for _, elem := range elems {
		s.Push(elem)
	}

	assert.Equal(t, elems[len(elems)-1], s.Top())
}

func TestRemoveLastElement(t *testing.T) {
	var (
		s     = NewStack("test")
		elems = makeElements(2)
	)
	for _, elem := range elems {
		s.Push(elem)
	}
	elem := s.RemoveAt(1)
	assert.Equal(t, elems[0], elem)
	assert.Nil(t, elem.prev)
	assert.Nil(t, elem.next)
	assert.Equal(t, 1, s.Len())
}

func TestIterAfterRemove(t *testing.T) {
	var (
		s     = NewStack("test")
		elems = makeElements(10)
	)
	for _, elem := range elems {
		s.Push(elem)
	}
	s.RemoveAt(0)

	i := 0
	s.Iter(func(elem *Element) {
		i++
	})
	assert.Equal(t, len(elems)-1, i)
}

func TestIteration(t *testing.T) {
	var (
		s     = NewStack("test")
		elems = makeElements(10)
	)
	for _, elem := range elems {
		s.Push(elem)
	}
	assert.Equal(t, len(elems), s.Len())

	i := 0
	s.Iter(func(elem *Element) {
		i++
	})
	assert.Equal(t, len(elems), i)
}

func TestPushVal(t *testing.T) {
	s := NewStack("test")

	// integer
	s.PushVal(2)
	elem := s.Pop()
	assert.Equal(t, int64(2), elem.BigInt().Int64())

	// byteArray
	s.PushVal([]byte("foo"))
	elem = s.Pop()
	assert.Equal(t, "foo", string(elem.Bytes()))

	// boolean
	s.PushVal(true)
	elem = s.Pop()
	assert.Equal(t, true, elem.Bool())

	// array
	s.PushVal([]StackItem{&BoolItem{true}, &BoolItem{false}, &BoolItem{true}})
	elem = s.Pop()
	assert.IsType(t, elem.value, &ArrayItem{})
}

func TestSwapElemValues(t *testing.T) {
	s := NewStack("test")

	s.PushVal(2)
	s.PushVal(4)

	a := s.Peek(0)
	b := s.Peek(1)

	// [ 4 ] -> a
	// [ 2 ] -> b

	aval := a.value
	bval := b.value
	a.value = bval
	b.value = aval

	// [ 2 ] -> a
	// [ 4 ] -> b

	assert.Equal(t, int64(2), s.Pop().BigInt().Int64())
	assert.Equal(t, int64(4), s.Pop().BigInt().Int64())
}

func makeElements(n int) []*Element {
	elems := make([]*Element, n)
	for i := 0; i < n; i++ {
		elems[i] = NewElement(i)
	}
	return elems
}
