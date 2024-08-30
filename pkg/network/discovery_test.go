package network

import (
	"errors"
	"net"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/network/capability"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeTransp struct {
	retFalse atomic.Int32
	started  atomic.Bool
	closed   atomic.Bool
	dialCh   chan string
	host     string
	port     string
}

type fakeAPeer struct {
	addr    string
	peer    string
	version *payload.Version
}

func (f *fakeAPeer) ConnectionAddr() string {
	return f.addr
}

func (f *fakeAPeer) PeerAddr() net.Addr {
	tcpAddr, err := net.ResolveTCPAddr("tcp", f.peer)
	if err != nil {
		panic(err)
	}
	return tcpAddr
}

func (f *fakeAPeer) Version() *payload.Version {
	return f.version
}

func newFakeTransp(s *Server, addr string) Transporter {
	tr := &fakeTransp{}
	h, p, err := net.SplitHostPort(addr)
	if err == nil {
		tr.host = h
		tr.port = p
	}
	return tr
}

func (ft *fakeTransp) Dial(addr string, timeout time.Duration) (AddressablePeer, error) {
	var ret error
	if ft.retFalse.Load() > 0 {
		ret = errors.New("smth bad happened")
	}
	ft.dialCh <- addr

	return &fakeAPeer{addr: addr, peer: addr}, ret
}
func (ft *fakeTransp) Accept() {
	if ft.started.Load() {
		panic("started twice")
	}
	ft.host = "0.0.0.0"
	ft.port = "42"
	ft.started.Store(true)
}
func (ft *fakeTransp) Proto() string {
	return ""
}
func (ft *fakeTransp) HostPort() (string, string) {
	return ft.host, ft.port
}
func (ft *fakeTransp) Close() {
	if ft.closed.Load() {
		panic("closed twice")
	}
	ft.closed.Store(true)
}
func TestDefaultDiscoverer(t *testing.T) {
	ts := &fakeTransp{}
	ts.dialCh = make(chan string)
	d := NewDefaultDiscovery(nil, time.Second/16, ts)

	tryMaxWait = 1 // Don't waste time.
	var set1 = []string{"1.1.1.1:10333", "2.2.2.2:10333"}
	slices.Sort(set1)

	// Added addresses should end up in the pool and in the unconnected set.
	// Done twice to check re-adding unconnected addresses, which should be
	// a no-op.
	for i := 0; i < 2; i++ {
		d.BackFill(set1...)
		assert.Equal(t, len(set1), d.PoolCount())
		set1D := d.UnconnectedPeers()
		slices.Sort(set1D)
		assert.Equal(t, 0, len(d.GoodPeers()))
		assert.Equal(t, 0, len(d.BadPeers()))
		require.Equal(t, set1, set1D)
	}
	require.Equal(t, 2, d.GetFanOut())

	// Request should make goroutines dial our addresses draining the pool.
	d.RequestRemote(len(set1))
	dialled := make([]string, 0)
	for i := 0; i < len(set1); i++ {
		select {
		case a := <-ts.dialCh:
			dialled = append(dialled, a)
			d.RegisterConnected(&fakeAPeer{addr: a, peer: a})
		case <-time.After(time.Second):
			t.Fatalf("timeout expecting for transport dial")
		}
	}
	require.Eventually(t, func() bool { return len(d.UnconnectedPeers()) == 0 }, 2*time.Second, 50*time.Millisecond)
	slices.Sort(dialled)
	assert.Equal(t, 0, d.PoolCount())
	assert.Equal(t, 0, len(d.BadPeers()))
	assert.Equal(t, 0, len(d.GoodPeers()))
	require.Equal(t, set1, dialled)

	// Registered good addresses should end up in appropriate set.
	for _, addr := range set1 {
		d.RegisterGood(&fakeAPeer{
			addr: addr,
			peer: addr,
			version: &payload.Version{
				Capabilities: capability.Capabilities{{
					Type: capability.FullNode,
					Data: &capability.Node{StartHeight: 123},
				}},
			},
		})
	}
	gAddrWithCap := d.GoodPeers()
	gAddrs := make([]string, len(gAddrWithCap))
	for i, addr := range gAddrWithCap {
		require.Equal(t, capability.Capabilities{
			{
				Type: capability.FullNode,
				Data: &capability.Node{StartHeight: 123},
			},
		}, addr.Capabilities)
		gAddrs[i] = addr.Address
	}
	slices.Sort(gAddrs)
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
		d.UnregisterConnected(&fakeAPeer{addr: addr, peer: addr}, false)
	}
	assert.Equal(t, 2, len(d.UnconnectedPeers())) // They're re-added automatically.
	assert.Equal(t, 0, len(d.BadPeers()))
	assert.Equal(t, len(set1), len(d.GoodPeers()))
	require.Equal(t, 2, d.PoolCount())

	// Now make Dial() fail and wait to see addresses in the bad list.
	ts.retFalse.Store(1)
	assert.Equal(t, len(set1), d.PoolCount())
	set1D := d.UnconnectedPeers()
	slices.Sort(set1D)
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
	require.Eventually(t, func() bool { return d.PoolCount() == 0 }, 2*time.Second, 50*time.Millisecond)
	slices.Sort(dialledBad)
	for i := 0; i < len(set1); i++ {
		for j := 0; j < connRetries; j++ {
			assert.Equal(t, set1[i], dialledBad[i*connRetries+j])
		}
	}
	require.Eventually(t, func() bool { return len(d.BadPeers()) == len(set1) }, 2*time.Second, 50*time.Millisecond)
	assert.Equal(t, 0, len(d.GoodPeers()))
	assert.Equal(t, 0, len(d.UnconnectedPeers()))

	// Re-adding bad addresses is a no-op.
	d.BackFill(set1...)
	assert.Equal(t, 0, len(d.UnconnectedPeers()))
	assert.Equal(t, len(set1), len(d.BadPeers()))
	assert.Equal(t, 0, len(d.GoodPeers()))
	require.Equal(t, 0, d.PoolCount())
}

func TestSeedDiscovery(t *testing.T) {
	var seeds = []string{"1.1.1.1:10333", "2.2.2.2:10333"}
	ts := &fakeTransp{}
	ts.dialCh = make(chan string)
	ts.retFalse.Store(1) // Fail all dial requests.
	slices.Sort(seeds)

	d := NewDefaultDiscovery(seeds, time.Second/10, ts)
	tryMaxWait = 1 // Don't waste time.

	d.RequestRemote(len(seeds))
	for i := 0; i < connRetries*2; i++ {
		for range seeds {
			select {
			case <-ts.dialCh:
			case <-time.After(time.Second):
				t.Fatalf("timeout expecting for transport dial")
			}
		}
	}
}
