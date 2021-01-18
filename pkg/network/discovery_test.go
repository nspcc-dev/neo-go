package network

import (
	"errors"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeTransp struct {
	retFalse int32
	dialCh   chan string
}

func (ft *fakeTransp) Dial(addr string, timeout time.Duration) error {
	ft.dialCh <- addr

	if atomic.LoadInt32(&ft.retFalse) > 0 {
		return errors.New("smth bad happened")
	}
	return nil
}
func (ft *fakeTransp) Accept() {
}
func (ft *fakeTransp) Proto() string {
	return ""
}
func (ft *fakeTransp) Close() {
}
func TestDefaultDiscoverer(t *testing.T) {
	ts := &fakeTransp{}
	ts.dialCh = make(chan string)
	d := NewDefaultDiscovery(nil, time.Second/2, ts)

	var set1 = []string{"1.1.1.1:10333", "2.2.2.2:10333"}
	sort.Strings(set1)

	// Added addresses should end up in the pool and in the unconnected set.
	// Done twice to check re-adding unconnected addresses, which should be
	// a no-op.
	for i := 0; i < 2; i++ {
		d.BackFill(set1...)
		assert.Equal(t, len(set1), d.PoolCount())
		set1D := d.UnconnectedPeers()
		sort.Strings(set1D)
		assert.Equal(t, 0, len(d.GoodPeers()))
		assert.Equal(t, 0, len(d.BadPeers()))
		require.Equal(t, set1, set1D)
	}

	// Request should make goroutines dial our addresses draining the pool.
	d.RequestRemote(len(set1))
	dialled := make([]string, 0)
	for i := 0; i < len(set1); i++ {
		select {
		case a := <-ts.dialCh:
			dialled = append(dialled, a)
			d.RegisterConnectedAddr(a)
		case <-time.After(time.Second):
			t.Fatalf("timeout expecting for transport dial")
		}
	}
	// Updated asynchronously.
	if len(d.UnconnectedPeers()) != 0 {
		time.Sleep(time.Second)
	}
	sort.Strings(dialled)
	assert.Equal(t, 0, d.PoolCount())
	assert.Equal(t, 0, len(d.UnconnectedPeers()))
	assert.Equal(t, 0, len(d.BadPeers()))
	assert.Equal(t, 0, len(d.GoodPeers()))
	require.Equal(t, set1, dialled)

	// Registered good addresses should end up in appropriate set.
	for _, addr := range set1 {
		d.RegisterGoodAddr(addr)
	}
	gAddrs := d.GoodPeers()
	sort.Strings(gAddrs)
	assert.Equal(t, 0, d.PoolCount())
	assert.Equal(t, 0, len(d.UnconnectedPeers()))
	assert.Equal(t, 0, len(d.BadPeers()))
	require.Equal(t, set1, gAddrs)

	// Re-adding connected addresses should be no-op.
	d.BackFill(set1...)
	assert.Equal(t, 0, len(d.UnconnectedPeers()))
	assert.Equal(t, 0, len(d.BadPeers()))
	assert.Equal(t, len(set1), len(d.GoodPeers()))
	require.Equal(t, 0, d.PoolCount())

	// Unregistering connected should work.
	for _, addr := range set1 {
		d.UnregisterConnectedAddr(addr)
	}
	assert.Equal(t, 0, len(d.UnconnectedPeers()))
	assert.Equal(t, 0, len(d.BadPeers()))
	assert.Equal(t, len(set1), len(d.GoodPeers()))
	require.Equal(t, 0, d.PoolCount())

	// Now make Dial() fail and wait to see addresses in the bad list.
	atomic.StoreInt32(&ts.retFalse, 1)
	d.BackFill(set1...)
	assert.Equal(t, len(set1), d.PoolCount())
	set1D := d.UnconnectedPeers()
	sort.Strings(set1D)
	assert.Equal(t, 0, len(d.BadPeers()))
	require.Equal(t, set1, set1D)

	dialledBad := make([]string, 0)
	d.RequestRemote(len(set1))
	for i := 0; i < connRetries; i++ {
		for j := 0; j < len(set1); j++ {
			select {
			case a := <-ts.dialCh:
				dialledBad = append(dialledBad, a)
			case <-time.After(time.Second):
				t.Fatalf("timeout expecting for transport dial; i: %d, j: %d", i, j)
			}
		}
	}
	require.Equal(t, 0, d.PoolCount())
	sort.Strings(dialledBad)
	for i := 0; i < len(set1); i++ {
		for j := 0; j < connRetries; j++ {
			assert.Equal(t, set1[i], dialledBad[i*connRetries+j])
		}
	}
	// Updated asynchronously.
	if len(d.BadPeers()) != len(set1) {
		time.Sleep(time.Second)
	}
	assert.Equal(t, len(set1), len(d.BadPeers()))
	assert.Equal(t, 0, len(d.GoodPeers()))
	assert.Equal(t, 0, len(d.UnconnectedPeers()))

	// Re-adding bad addresses is a no-op.
	d.BackFill(set1...)
	assert.Equal(t, 0, len(d.UnconnectedPeers()))
	assert.Equal(t, len(set1), len(d.BadPeers()))
	assert.Equal(t, 0, len(d.GoodPeers()))
	require.Equal(t, 0, d.PoolCount())

	// Close should work and subsequent RequestRemote is a no-op.
	d.Close()
	d.RequestRemote(42)
}

func TestSeedDiscovery(t *testing.T) {
	var seeds = []string{"1.1.1.1:10333", "2.2.2.2:10333"}
	ts := &fakeTransp{}
	ts.dialCh = make(chan string)
	atomic.StoreInt32(&ts.retFalse, 1) // Fail all dial requests.
	sort.Strings(seeds)

	d := NewDefaultDiscovery(seeds, time.Second/10, ts)

	d.RequestRemote(len(seeds))
	dialled := make([]string, 0)
	for i := 0; i < connRetries*2; i++ {
		for range seeds {
			select {
			case a := <-ts.dialCh:
				dialled = append(dialled, a)
			case <-time.After(time.Second):
				t.Fatalf("timeout expecting for transport dial")
			}
		}
	}
}
