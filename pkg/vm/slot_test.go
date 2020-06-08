package vm

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func TestSlot_Get(t *testing.T) {
	s := newSlot(3, newRefCounter())
	require.NotNil(t, s)
	require.Equal(t, 3, s.Size())

	// Null is the default
	item := s.Get(2)
	require.Equal(t, stackitem.Null{}, item)

	s.Set(1, stackitem.NewBigInteger(big.NewInt(42)))
	require.Equal(t, stackitem.NewBigInteger(big.NewInt(42)), s.Get(1))
}
