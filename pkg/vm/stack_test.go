package vm

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPushElement(t *testing.T) {
	elems := makeElements(10)
	s := NewStack("test")
	for _, elem := range elems {
		s.Push(elem)
	}

	assert.Equal(t, len(elems), s.Len())

	for i := range elems {
		assert.Equal(t, elems[len(elems)-1-i], s.Peek(i))
	}
}

func TestStack_PushVal(t *testing.T) {
	type (
		i32      int32
		testByte uint8
	)

	s := NewStack("test")
	require.NotPanics(t, func() { s.PushVal(i32(123)) })
	require.NotPanics(t, func() { s.PushVal(testByte(42)) })
	require.Equal(t, 2, s.Len())
	require.Equal(t, big.NewInt(42), s.Pop().Value())
	require.Equal(t, big.NewInt(123), s.Pop().Value())
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
	s.Iter(func(_ Element) {
		i++
	})
	assert.Equal(t, len(elems)-1, i)
}

func TestIteration(t *testing.T) {
	var (
		n     = 10
		s     = NewStack("test")
		elems = makeElements(n)
	)
	for _, elem := range elems {
		s.Push(elem)
	}
	assert.Equal(t, len(elems), s.Len())

	iteratedElems := make([]Element, 0)

	s.Iter(func(elem Element) {
		iteratedElems = append(iteratedElems, elem)
	})

	// Top to bottom order of iteration.
	poppedElems := make([]Element, 0)
	for s.Len() != 0 {
		poppedElems = append(poppedElems, s.Pop())
	}
	assert.Equal(t, poppedElems, iteratedElems)
}

func TestBackIteration(t *testing.T) {
	var (
		n     = 10
		s     = NewStack("test")
		elems = makeElements(n)
	)
	for _, elem := range elems {
		s.Push(elem)
	}
	assert.Equal(t, len(elems), s.Len())

	iteratedElems := make([]Element, 0)

	s.IterBack(func(elem Element) {
		iteratedElems = append(iteratedElems, elem)
	})
	// Bottom to the top order of iteration.
	assert.Equal(t, elems, iteratedElems)
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
	s.PushVal([]stackitem.Item{stackitem.NewBool(true), stackitem.NewBool(false), stackitem.NewBool(true)})
	elem = s.Pop()
	assert.IsType(t, elem.value, &stackitem.Array{})
}

func TestStack_ToArray(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		s := NewStack("test")
		items := s.ToArray()
		require.Equal(t, 0, len(items))
	})
	t.Run("NonEmpty", func(t *testing.T) {
		s := NewStack("test")
		expected := []stackitem.Item{stackitem.Make(1), stackitem.Make(true)}
		for i := range expected {
			s.PushVal(expected[i])
		}
		require.Equal(t, expected, s.ToArray())
	})
}

func TestSwapElemValues(t *testing.T) {
	s := NewStack("test")

	s.PushVal(2)
	s.PushVal(4)

	assert.NoError(t, s.Swap(0, 1))
	assert.Equal(t, int64(2), s.Pop().BigInt().Int64())
	assert.Equal(t, int64(4), s.Pop().BigInt().Int64())

	s.PushVal(1)
	s.PushVal(2)
	s.PushVal(3)
	s.PushVal(4)

	assert.NoError(t, s.Swap(1, 3))
	assert.Equal(t, int64(4), s.Pop().BigInt().Int64())
	assert.Equal(t, int64(1), s.Pop().BigInt().Int64())
	assert.Equal(t, int64(2), s.Pop().BigInt().Int64())
	assert.Equal(t, int64(3), s.Pop().BigInt().Int64())

	s.PushVal(1)
	s.PushVal(2)
	s.PushVal(3)
	s.PushVal(4)

	assert.Error(t, s.Swap(-1, 0))
	assert.Error(t, s.Swap(0, -3))
	assert.Error(t, s.Swap(0, 4))
	assert.Error(t, s.Swap(5, 0))

	assert.NoError(t, s.Swap(1, 1))
	assert.Equal(t, int64(4), s.Pop().BigInt().Int64())
	assert.Equal(t, int64(3), s.Pop().BigInt().Int64())
	assert.Equal(t, int64(2), s.Pop().BigInt().Int64())
	assert.Equal(t, int64(1), s.Pop().BigInt().Int64())
}

func TestRoll(t *testing.T) {
	s := NewStack("test")

	s.PushVal(1)
	s.PushVal(2)
	s.PushVal(3)
	s.PushVal(4)

	assert.NoError(t, s.Roll(2))
	assert.Equal(t, int64(2), s.Pop().BigInt().Int64())
	assert.Equal(t, int64(4), s.Pop().BigInt().Int64())
	assert.Equal(t, int64(3), s.Pop().BigInt().Int64())
	assert.Equal(t, int64(1), s.Pop().BigInt().Int64())

	s.PushVal(1)
	s.PushVal(2)
	s.PushVal(3)
	s.PushVal(4)

	assert.NoError(t, s.Roll(3))
	assert.Equal(t, int64(1), s.Pop().BigInt().Int64())
	assert.Equal(t, int64(4), s.Pop().BigInt().Int64())
	assert.Equal(t, int64(3), s.Pop().BigInt().Int64())
	assert.Equal(t, int64(2), s.Pop().BigInt().Int64())

	s.PushVal(1)
	s.PushVal(2)
	s.PushVal(3)
	s.PushVal(4)

	assert.Error(t, s.Roll(-1))
	assert.Error(t, s.Roll(4))

	assert.NoError(t, s.Roll(0))
	assert.Equal(t, int64(4), s.Pop().BigInt().Int64())
	assert.Equal(t, int64(3), s.Pop().BigInt().Int64())
	assert.Equal(t, int64(2), s.Pop().BigInt().Int64())
	assert.Equal(t, int64(1), s.Pop().BigInt().Int64())
}

func TestInsertAt(t *testing.T) {
	s := NewStack("stack")
	s.PushVal(1)
	s.PushVal(2)
	s.PushVal(3)
	s.PushVal(4)
	s.PushVal(5)

	e := s.Dup(1) // it's `4`
	s.InsertAt(e, 3)

	assert.Equal(t, int64(5), s.Peek(0).BigInt().Int64())
	assert.Equal(t, int64(4), s.Peek(1).BigInt().Int64())
	assert.Equal(t, int64(3), s.Peek(2).BigInt().Int64())
	assert.Equal(t, int64(4), s.Peek(3).BigInt().Int64())
	assert.Equal(t, int64(2), s.Peek(4).BigInt().Int64())
	assert.Equal(t, int64(1), s.Peek(5).BigInt().Int64())
}

func TestPopSigElements(t *testing.T) {
	s := NewStack("test")

	_, err := s.PopSigElements()
	assert.NotNil(t, err)

	s.PushVal([]stackitem.Item{})
	_, err = s.PopSigElements()
	assert.NotNil(t, err)

	s.PushVal([]stackitem.Item{stackitem.NewBool(false)})
	_, err = s.PopSigElements()
	assert.NotNil(t, err)

	b1 := []byte("smth")
	b2 := []byte("strange")
	s.PushVal([]stackitem.Item{stackitem.NewByteArray(b1), stackitem.NewByteArray(b2)})
	z, err := s.PopSigElements()
	assert.Nil(t, err)
	assert.Equal(t, z, [][]byte{b1, b2})

	s.PushVal(2)
	_, err = s.PopSigElements()
	assert.NotNil(t, err)

	s.PushVal(b1)
	s.PushVal(2)
	_, err = s.PopSigElements()
	assert.NotNil(t, err)

	s.PushVal(b2)
	s.PushVal(b1)
	s.PushVal(2)
	z, err = s.PopSigElements()
	assert.Nil(t, err)
	assert.Equal(t, z, [][]byte{b1, b2})
}

func makeElements(n int) []Element {
	elems := make([]Element, n)
	for i := range n {
		elems[i] = NewElement(i)
	}
	return elems
}
