package mptpool

import (
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestAddRemoveUpdate(t *testing.T) {
	mp := New()

	i1 := []byte{1, 2, 3}
	i1h := util.Uint256{1, 2, 3}
	i2 := []byte{2, 3, 4}
	i2h := util.Uint256{2, 3, 4}
	i3 := []byte{4, 5, 6}
	i3h := util.Uint256{3, 4, 5}
	i4 := []byte{3, 4, 5} // has the same hash as i3
	i5 := []byte{6, 7, 8} // has the same hash as i3
	mapAll := map[util.Uint256][][]byte{i1h: {i1}, i2h: {i2}, i3h: {i4, i3}}

	// No items
	_, ok := mp.TryGet(i1h)
	require.False(t, ok)
	require.False(t, mp.ContainsKey(i1h))
	require.Equal(t, 0, mp.Count())
	require.Equal(t, map[util.Uint256][][]byte{}, mp.GetAll())

	// Add i1, i2, check OK
	mp.Add(i1h, i1)
	mp.Add(i2h, i2)
	itm, ok := mp.TryGet(i1h)
	require.True(t, ok)
	require.Equal(t, [][]byte{i1}, itm)
	require.True(t, mp.ContainsKey(i1h))
	require.True(t, mp.ContainsKey(i2h))
	require.Equal(t, map[util.Uint256][][]byte{i1h: {i1}, i2h: {i2}}, mp.GetAll())
	require.Equal(t, 2, mp.Count())

	// Remove i1 and unexisting item
	mp.Remove(i3h)
	mp.Remove(i1h)
	require.False(t, mp.ContainsKey(i1h))
	require.True(t, mp.ContainsKey(i2h))
	require.Equal(t, map[util.Uint256][][]byte{i2h: {i2}}, mp.GetAll())
	require.Equal(t, 1, mp.Count())

	// Update: remove nothing, add all
	mp.Update(nil, mapAll)
	require.Equal(t, mapAll, mp.GetAll())
	require.Equal(t, 3, mp.Count())
	// Update: remove all, add all
	mp.Update(mapAll, mapAll)
	require.Equal(t, mapAll, mp.GetAll()) // deletion first, addition after that
	require.Equal(t, 3, mp.Count())
	// Update: remove all, add nothing
	mp.Update(mapAll, nil)
	require.Equal(t, map[util.Uint256][][]byte{}, mp.GetAll())
	require.Equal(t, 0, mp.Count())
	// Update: remove several, add several
	mp.Update(map[util.Uint256][][]byte{i1h: {i1}, i2h: {i2}}, map[util.Uint256][][]byte{i2h: {i2}, i3h: {i3}})
	require.Equal(t, map[util.Uint256][][]byte{i2h: {i2}, i3h: {i3}}, mp.GetAll())
	require.Equal(t, 2, mp.Count())

	// Update: remove nothing, add several with same hashes
	mp.Update(nil, map[util.Uint256][][]byte{i3h: {i5, i4}}) // should be sorted by the pool
	require.Equal(t, map[util.Uint256][][]byte{i2h: {i2}, i3h: {i4, i3, i5}}, mp.GetAll())
	require.Equal(t, 2, mp.Count())
	// Update: remove several with same hashes, add nothing
	mp.Update(map[util.Uint256][][]byte{i3h: {i5, i4}}, nil)
	require.Equal(t, map[util.Uint256][][]byte{i2h: {i2}, i3h: {i3}}, mp.GetAll())
	require.Equal(t, 2, mp.Count())
	// Update: remove several with same hashes, add several with same hashes
	mp.Update(map[util.Uint256][][]byte{i3h: {i5, i3}}, map[util.Uint256][][]byte{i3h: {i5, i4}})
	require.Equal(t, map[util.Uint256][][]byte{i2h: {i2}, i3h: {i4, i5}}, mp.GetAll())
	require.Equal(t, 2, mp.Count())
}

func TestRetransmission(t *testing.T) {
	type notification struct {
		hash     util.Uint256
		count    int
		finished bool
	}
	buffer := make(map[util.Uint256]notification)
	resentCh := make(chan notification)
	resentThreshold := 100 * time.Millisecond
	resentAttempts := 3

	mp := New()
	mp.SetResendThreshold(resentThreshold, func(m map[util.Uint256]bool) {
		for h := range m {
			att, ok := buffer[h]
			if !ok {
				att.hash = h
			}
			if !att.finished {
				if att.count < resentAttempts {
					att.count++
				} else {
					att.finished = true
				}
				buffer[h] = att
				resentCh <- att
			}
		}
	})
	require.False(t, mp.retransmissionOn.Load())
	go mp.ResendStaleItems()
	require.Eventually(t, mp.retransmissionOn.Load, time.Second, 10*time.Millisecond)

	// should immediately return
	mp.ResendStaleItems()

	i1 := []byte{1, 2, 3}
	i1h := util.Uint256{1, 2, 3}
	i2 := []byte{2, 3, 4}
	i2h := util.Uint256{2, 3, 4}

	mp.Update(nil, map[util.Uint256][][]byte{i1h: {i1}, i2h: {i2}})
	for i := 0; i < resentAttempts; i++ {
		itm1 := <-resentCh
		itm2 := <-resentCh
		require.ElementsMatch(t, []notification{
			{
				hash:     i1h,
				count:    i + 1,
				finished: false,
			},
			{
				hash:     i2h,
				count:    i + 1,
				finished: false,
			},
		}, []notification{itm1, itm2})
	}
	itm1 := <-resentCh
	itm2 := <-resentCh
	require.ElementsMatch(t, []notification{
		{
			hash:     i1h,
			count:    resentAttempts,
			finished: true,
		},
		{
			hash:     i2h,
			count:    resentAttempts,
			finished: true,
		},
	}, []notification{itm1, itm2})

	// ResendStaleItems routine should be stopped
	mp.Update(map[util.Uint256][][]byte{i1h: {i1}, i2h: {i2}}, nil)
	require.Equal(t, 0, mp.Count())
	require.Eventually(t, func() bool {
		return !mp.retransmissionOn.Load()
	}, time.Second, 10*time.Millisecond)
}
