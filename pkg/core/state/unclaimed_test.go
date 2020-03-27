package state

import (
	"encoding/binary"
	"math/rand"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestUnclaimedBalance_Structure(t *testing.T) {
	b := randomUnclaimed(t)
	buf, err := testserdes.EncodeBinary(b)
	require.NoError(t, err)
	require.Equal(t, UnclaimedBalanceSize, len(buf))
	require.Equal(t, b.Tx.BytesBE(), buf[:util.Uint256Size])
	require.Equal(t, b.Index, binary.LittleEndian.Uint16(buf[util.Uint256Size:]))
}

func TestUnclaimedBalances_Put(t *testing.T) {
	bs := new(UnclaimedBalances)
	b1 := randomUnclaimed(t)
	b2 := randomUnclaimed(t)
	b3 := randomUnclaimed(t)

	require.NoError(t, bs.Put(b1))
	require.Equal(t, 1, bs.Size())
	require.NoError(t, bs.Put(b2))
	require.Equal(t, 2, bs.Size())
	require.NoError(t, bs.Put(b3))
	require.Equal(t, 3, bs.Size())
	require.True(t, bs.Remove(b2.Tx, b2.Index))
	require.Equal(t, 2, bs.Size())
	require.False(t, bs.Remove(b2.Tx, b2.Index))
	require.Equal(t, 2, bs.Size())
	require.True(t, bs.Remove(b1.Tx, b1.Index))
	require.Equal(t, 1, bs.Size())
	require.True(t, bs.Remove(b3.Tx, b3.Index))
	require.Equal(t, 0, bs.Size())
}

func TestUnclaimedBalances_ForEach(t *testing.T) {
	bs := new(UnclaimedBalances)
	b1 := randomUnclaimed(t)
	b2 := randomUnclaimed(t)
	b3 := randomUnclaimed(t)

	require.NoError(t, bs.Put(b1))
	require.NoError(t, bs.Put(b2))
	require.NoError(t, bs.Put(b3))

	var indices []uint16
	err := bs.ForEach(func(b *UnclaimedBalance) error {
		indices = append(indices, b.Index)
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, []uint16{b1.Index, b2.Index, b3.Index}, indices)
}

func randomUnclaimed(t *testing.T) *UnclaimedBalance {
	b := new(UnclaimedBalance)
	b.Tx = random.Uint256()
	b.Index = uint16(rand.Uint32())
	b.Start = rand.Uint32()
	b.End = rand.Uint32()
	b.Value = util.Fixed8(rand.Int63())

	return b
}
