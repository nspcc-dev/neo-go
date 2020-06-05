package cache

import (
	"math/rand"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestRelayCache_Add(t *testing.T) {
	const capacity = 3
	payloads := getDifferentItems(t, capacity+1)
	c := NewFIFOCache(capacity)
	require.Equal(t, 0, c.queue.Len())
	require.Equal(t, 0, len(c.elems))

	for i := 1; i < capacity; i++ {
		c.Add(&payloads[i])
		require.True(t, c.Has(payloads[i].Hash()))
		require.Equal(t, i, c.queue.Len())
		require.Equal(t, i, len(c.elems))
	}

	// add already existing payload
	c.Add(&payloads[1])
	require.Equal(t, capacity-1, c.queue.Len())
	require.Equal(t, capacity-1, len(c.elems))

	c.Add(&payloads[0])
	require.Equal(t, capacity, c.queue.Len())
	require.Equal(t, capacity, len(c.elems))

	// capacity does not exceed maximum
	c.Add(&payloads[capacity])
	require.Equal(t, capacity, c.queue.Len())
	require.Equal(t, capacity, len(c.elems))

	// recent payloads are still present in cache
	require.Equal(t, &payloads[0], c.Get(payloads[0].Hash()))
	for i := 2; i <= capacity; i++ {
		require.Equal(t, &payloads[i], c.Get(payloads[i].Hash()))
	}

	// oldest payload was removed
	require.Equal(t, nil, c.Get(payloads[1].Hash()))
}

type testHashable []byte

// Hash implements Hashable.
func (h testHashable) Hash() util.Uint256 { return hash.Sha256(h) }

func getDifferentItems(t *testing.T, n int) []testHashable {
	items := make([]testHashable, n)
	for i := range items {
		items[i] = random.Bytes(rand.Int() % 10)
	}
	return items
}
