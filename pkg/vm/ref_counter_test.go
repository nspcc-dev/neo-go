package vm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRefCounter_Add(t *testing.T) {
	r := newRefCounter()

	require.Equal(t, 0, r.size)

	r.Add(NullItem{})
	require.Equal(t, 1, r.size)

	r.Add(NullItem{})
	require.Equal(t, 2, r.size) // count scalar items twice

	arr := NewArrayItem([]StackItem{NewByteArrayItem([]byte{1}), NewBoolItem(false)})
	r.Add(arr)
	require.Equal(t, 5, r.size) // array + 2 elements

	r.Add(arr)
	require.Equal(t, 6, r.size) // count only array

	r.Remove(arr)
	require.Equal(t, 5, r.size)

	r.Remove(arr)
	require.Equal(t, 2, r.size)
}
