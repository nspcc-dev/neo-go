package consensus

import (
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/stretchr/testify/require"
)

func TestRelayCache_Add(t *testing.T) {
	const capacity = 3
	payloads := getDifferentPayloads(t, capacity+1)
	c := newFIFOCache(capacity)
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

func getDifferentPayloads(t *testing.T, n int) (payloads []Payload) {
	payloads = make([]Payload, n)
	for i := range payloads {
		var sign [signatureSize]byte
		random.Fill(sign[:])

		payloads[i].message.ValidatorIndex = byte(i)
		payloads[i].message.Type = commitType
		payloads[i].payload = &commit{
			signature: sign,
		}
		payloads[i].encodeData()
	}

	return
}
