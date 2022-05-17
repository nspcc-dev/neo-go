package vm

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func TestRefCounter_Add(t *testing.T) {
	r := newRefCounter()

	require.Equal(t, 0, int(*r))

	r.Add(stackitem.Null{})
	require.Equal(t, 1, int(*r))

	r.Add(stackitem.Null{})
	require.Equal(t, 2, int(*r)) // count scalar items twice

	arr := stackitem.NewArray([]stackitem.Item{stackitem.NewByteArray([]byte{1}), stackitem.NewBool(false)})
	r.Add(arr)
	require.Equal(t, 5, int(*r)) // array + 2 elements

	r.Add(arr)
	require.Equal(t, 6, int(*r)) // count only array

	r.Remove(arr)
	require.Equal(t, 5, int(*r))

	r.Remove(arr)
	require.Equal(t, 2, int(*r))

	m := stackitem.NewMap()
	m.Add(stackitem.NewByteArray([]byte("some")), stackitem.NewBool(false))
	r.Add(m)
	require.Equal(t, 5, int(*r)) // map + key + value

	r.Add(m)
	require.Equal(t, 6, int(*r)) // map only

	r.Remove(m)
	require.Equal(t, 5, int(*r))

	r.Remove(m)
	require.Equal(t, 2, int(*r))
}

func BenchmarkRefCounter_Add(b *testing.B) {
	a := stackitem.NewArray(nil)
	rc := newRefCounter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rc.Add(a)
	}
}
