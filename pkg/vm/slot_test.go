package vm

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSlot_Get(t *testing.T) {
	s := newSlot(3)
	require.NotNil(t, s)
	require.Equal(t, 3, s.Size())

	// NullItem is the default
	item := s.Get(2)
	require.Equal(t, NullItem{}, item)

	s.Set(1, NewBigIntegerItem(big.NewInt(42)))
	require.Equal(t, NewBigIntegerItem(big.NewInt(42)), s.Get(1))
}
