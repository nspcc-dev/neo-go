package bitfield

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFields(t *testing.T) {
	a := New(128)
	b := New(128)
	a.Set(10)
	b.Set(10)
	a.Set(42)
	b.Set(42)
	a.Set(100)
	b.Set(100)
	require.True(t, a.IsSet(42))
	require.False(t, b.IsSet(43))
	require.True(t, a.IsSubset(b))

	v := uint64(1<<10 | 1<<42)
	require.Equal(t, v, a[0])
	require.Equal(t, v, b[0])

	require.True(t, a.Equals(b))

	c := a.Copy()
	require.True(t, c.Equals(b))

	z := New(128)
	require.True(t, z.IsSubset(c))
	c.And(a)
	require.True(t, c.Equals(b))
	c.And(z)
	require.True(t, c.Equals(z))

	c = New(64)
	require.False(t, z.IsSubset(c))
	c[0] = a[0]
	require.False(t, c.Equals(a))
	require.True(t, c.IsSubset(a))

	b.And(c)
	require.False(t, b.Equals(a))
}
