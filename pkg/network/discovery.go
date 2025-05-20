package network

import (
	"fmt"
	"go.uber.org/zap"
	"math"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/network/capability"
)

const (
	maxPoolSize = 10000
	connRetries = 3
)

var (
	// Maximum waiting time before connection attempt.
	tryMaxWait = time.Second / 2
)

// Discoverer is an interface that is responsible for maintaining
// a healthy connection pool.
type Discoverer interface {
	BackFill(...string)
	GetFanOut() int
	NetworkSize() int
	PoolCount() int
	RequestRemote(int)
	RegisterSelf(AddressablePeer)
	RegisterGood(AddressablePeer)
	RegisterConnected(AddressablePeer)
	UnregisterConnected(AddressablePeer, bool)
	UnconnectedPeers() []string
	BadPeers() []string
	GoodPeers() []AddressWithCapabilities
}

// AddressWithCapabilities represents a node address with its capabilities.
type AddressWithCapabilities struct {
	Address      string
	Capabilities capability.Capabilities
}

// DefaultDiscovery default implementation of the Discoverer interface.
type DefaultDiscovery struct {
	seeds            map[string]string
	transport        Transporter
	lock             sync.RWMutex
	dialTimeout      time.Duration
	badAddrs         map[string]bool
	connectedAddrs   map[string]bool
	handshakedAddrs  map[string]bool
	goodAddrs        map[string]capability.Capabilities
	unconnectedAddrs map[string]int
	attempted        map[string]bool
	outstanding      int32
	optimalFanOut    int32
	networkSize      int32
	requestCh        chan int
	log              *zap.Logger
}

// NewDefaultDiscovery returns a new DefaultDiscovery.
func NewDefaultDiscovery(addrs []string, dt time.Duration, ts Transporter, log *zap.Logger) *DefaultDiscovery {
	var seeds = make(map[string]string)
	for i := range addrs {
		seeds[addrs[i]] = ""
	}
	d := &DefaultDiscovery{
		seeds:            seeds,
		transport:        ts,
		dialTimeout:      dt,
		badAddrs:         make(map[string]bool),
		connectedAddrs:   make(map[string]bool),
		handshakedAddrs:  make(map[string]bool),
		goodAddrs:        make(map[string]capability.Capabilities),
		unconnectedAddrs: make(map[string]int),
		attempted:        make(map[string]bool),
		requestCh:        make(chan int),
		log:              log,
	}
	return d
}

func newDefaultDiscovery(addrs []string, dt time.Duration, ts Transporter, log *zap.Logger) Discoverer {
	return NewDefaultDiscovery(addrs, dt, ts, log)
}

// BackFill implements the Discoverer interface and will backfill
// the pool with the given addresses.
func (d *DefaultDiscovery) BackFill(addrs ...string) {
	d.lock.Lock()
	d.backfill(addrs...)
	d.lock.Unlock()
}

func (d *DefaultDiscovery) backfill(addrs ...string) {
	d.log.Info("backfilling addresses", zap.Strings("addrs", addrs))
	for _, addr := range addrs {
		if d.badAddrs[addr] || d.connectedAddrs[addr] || d.handshakedAddrs[addr] ||
			d.unconnectedAddrs[addr] > 0 {
			continue
		}
		d.pushToPoolOrDrop(addr)
	}
	d.updateNetSize()
}

// PoolCount returns the number of the available node addresses.
func (d *DefaultDiscovery) PoolCount() int {
	d.lock.RLock()
	defer d.lock.RUnlock()
	return d.poolCount()
}

func (d *DefaultDiscovery) poolCount() int {
	return len(d.unconnectedAddrs)
}

// pushToPoolOrDrop tries to push the address given into the pool, but if the pool
// is already full, it just drops it.
func (d *DefaultDiscovery) pushToPoolOrDrop(addr string) {
	if len(d.unconnectedAddrs) < maxPoolSize {
		d.unconnectedAddrs[addr] = connRetries
		d.log.Info("pushing new address to unconnectedAddrs pool", zap.String("addr", addr))
	}
}

// RequestRemote tries to establish a connection with n nodes.
func (d *DefaultDiscovery) RequestRemote(requested int) {
	outstanding := int(atomic.LoadInt32(&d.outstanding))
	d.log.Info("requesting remote", zap.Int("requested", requested), zap.Int("outstanding", outstanding))
	requested -= outstanding
	for ; requested > 0; requested-- {
		d.log.Info("requesting remote address", zap.Int("requested", requested), zap.Int("outstanding", outstanding))
		var nextAddr string
		d.lock.Lock()
		for addr := range d.unconnectedAddrs {
			d.log.Info(fmt.Sprintf("unconnected address %s", addr),
				zap.Bool("connected", d.connectedAddrs[addr]),
				zap.Bool("handshaked", d.handshakedAddrs[addr]),
				zap.Bool("attempted", d.attempted[addr]))
			if !d.connectedAddrs[addr] && !d.handshakedAddrs[addr] && !d.attempted[addr] {
				d.log.Info("choosing peer from unconnectedAddrs", zap.String("addr", addr), zap.Int("requested", requested))
				nextAddr = addr
				break
			}
		}

		if nextAddr == "" {
			// Empty pool, try seeds.
			for addr, ip := range d.seeds {
				d.log.Info(fmt.Sprintf("seed address %s", addr),
					zap.String("ip", ip),
					zap.Bool("attempted", d.attempted[addr]))
				if ip == "" && !d.attempted[addr] {
					d.log.Info("choosing peer from seeds", zap.String("addr", addr), zap.Int("requested", requested))
					nextAddr = addr
					break
				}
			}
		}
		if nextAddr == "" {
			d.lock.Unlock()
			d.log.Info("dropping request", zap.Int("requested", requested))
			// The pool is empty, but all seed nodes are already connected (or attempted),
			// we can end up in an infinite loop here, so drop the request.
			break
		}
		d.attempted[nextAddr] = true
		d.lock.Unlock()
		newOut := atomic.AddInt32(&d.outstanding, 1)
		d.log.Info("updating outstanding", zap.Int32("new", newOut), zap.String("delta", "+1"))
		go d.tryAddress(nextAddr)
	}
}

// RegisterSelf registers the given Peer as a bad one, because it's our own node.
func (d *DefaultDiscovery) RegisterSelf(p AddressablePeer) {
	var connaddr = p.ConnectionAddr()
	d.lock.Lock()
	delete(d.connectedAddrs, connaddr)
	d.registerBad(connaddr, true)
	d.registerBad(p.PeerAddr().String(), true)
	d.lock.Unlock()
}

func (d *DefaultDiscovery) registerBad(addr string, force bool) {
	d.log.Info("register bad address", zap.String("addr", addr), zap.Bool("force", force))
	old, isSeed := d.seeds[addr]
	if isSeed {
		if !force {
			d.seeds[addr] = ""
			d.log.Info("updating seed address", zap.String("seed", addr),
				zap.String("old", old), zap.String("new", ""))
		} else {
			d.seeds[addr] = "forever" // That's our own address, so never try connecting to it.
			d.log.Info("updating seed address", zap.String("seed", addr),
				zap.String("old", old), zap.String("new", "forever"))
		}
	} else {
		d.unconnectedAddrs[addr]--
		if d.unconnectedAddrs[addr] <= 0 || force {
			d.log.Info("new bad address", zap.String("addr", addr), zap.Bool("force", force))
			d.badAddrs[addr] = true
			delete(d.unconnectedAddrs, addr)
			delete(d.goodAddrs, addr)
		}
	}
	d.updateNetSize()
}

// UnconnectedPeers returns all addresses of unconnected addrs.
func (d *DefaultDiscovery) UnconnectedPeers() []string {
	d.lock.RLock()
	addrs := make([]string, 0, len(d.unconnectedAddrs))
	for addr := range d.unconnectedAddrs {
		addrs = append(addrs, addr)
	}
	d.lock.RUnlock()
	return addrs
}

// BadPeers returns all addresses of bad addrs.
func (d *DefaultDiscovery) BadPeers() []string {
	d.lock.RLock()
	addrs := make([]string, 0, len(d.badAddrs))
	for addr := range d.badAddrs {
		addrs = append(addrs, addr)
	}
	d.lock.RUnlock()
	return addrs
}

// GoodPeers returns all addresses of known good peers (that at least once
// succeeded handshaking with us).
func (d *DefaultDiscovery) GoodPeers() []AddressWithCapabilities {
	d.lock.RLock()
	addrs := make([]AddressWithCapabilities, 0, len(d.goodAddrs))
	for addr, cap := range d.goodAddrs {
		addrs = append(addrs, AddressWithCapabilities{
			Address:      addr,
			Capabilities: cap,
		})
	}
	d.lock.RUnlock()
	return addrs
}

// RegisterGood registers a known good connected peer that has passed
// handshake successfully.
func (d *DefaultDiscovery) RegisterGood(p AddressablePeer) {
	var s = p.PeerAddr().String()
	d.lock.Lock()
	d.log.Info("register good address", zap.String("addr", s))
	d.handshakedAddrs[s] = true
	d.goodAddrs[s] = p.Version().Capabilities
	delete(d.badAddrs, s)
	d.lock.Unlock()
}

// UnregisterConnected tells the discoverer that this peer is no longer
// connected, but it is still considered a good one.
func (d *DefaultDiscovery) UnregisterConnected(p AddressablePeer, duplicate bool) {
	var (
		peeraddr = p.PeerAddr().String()
		connaddr = p.ConnectionAddr()
	)
	d.lock.Lock()
	d.log.Info("unregister connected", zap.String("addr", peeraddr), zap.String("connaddr", connaddr), zap.Bool("duplicate", duplicate))
	delete(d.connectedAddrs, connaddr)
	if !duplicate {
		for addr, ip := range d.seeds {
			if ip == peeraddr {
				d.seeds[addr] = ""
				d.log.Info("updating seed address", zap.String("seed", addr),
					zap.String("old", ip), zap.String("new", ""))
				break
			}
		}
		delete(d.handshakedAddrs, peeraddr)
		if _, ok := d.goodAddrs[peeraddr]; ok {
			d.backfill(peeraddr)
		}
	}
	d.lock.Unlock()
}

// RegisterConnected tells discoverer that the given peer is now connected.
func (d *DefaultDiscovery) RegisterConnected(p AddressablePeer) {
	var addr = p.ConnectionAddr()
	d.lock.Lock()
	d.registerConnected(addr)
	d.lock.Unlock()
}

func (d *DefaultDiscovery) registerConnected(addr string) {
	d.log.Info("register connected", zap.String("addr", addr))
	delete(d.unconnectedAddrs, addr)
	d.connectedAddrs[addr] = true
	d.updateNetSize()
}

// GetFanOut returns the optimal number of nodes to broadcast packets to.
func (d *DefaultDiscovery) GetFanOut() int {
	return int(atomic.LoadInt32(&d.optimalFanOut))
}

// NetworkSize returns the estimated network size.
func (d *DefaultDiscovery) NetworkSize() int {
	return int(atomic.LoadInt32(&d.networkSize))
}

// updateNetSize updates network size estimation metric. Must be called under read lock.
func (d *DefaultDiscovery) updateNetSize() {
	var netsize = len(d.handshakedAddrs) + len(d.unconnectedAddrs) + 1 // 1 for the node itself.
	var fanOut = 2.5 * math.Log(float64(netsize-1))                    // -1 for the number of potential peers.
	if netsize == 2 {                                                  // log(1) == 0.
		fanOut = 1 // But we still want to push messages to the peer.
	}

	newFOut := int32(fanOut + 0.5)
	atomic.StoreInt32(&d.optimalFanOut, newFOut) // Truncating conversion, hence +0.5.
	atomic.StoreInt32(&d.networkSize, int32(netsize))
	poolCnt := d.poolCount()
	d.log.Info("updating net size", zap.Int32("fanout", newFOut), zap.Int32("size", int32(netsize)), zap.Int("poolCnt", poolCnt))
	updateNetworkSizeMetric(netsize)
	updatePoolCountMetric(poolCnt)
}

func (d *DefaultDiscovery) tryAddress(addr string) {
	d.log.Info("dealing address", zap.String("addr", addr))
	var tout = rand.Int64N(int64(tryMaxWait))
	time.Sleep(time.Duration(tout)) // Have a sleep before working hard.
	p, err := d.transport.Dial(addr, d.dialTimeout)
	newOut := atomic.AddInt32(&d.outstanding, -1)
	d.log.Info("updating outstanding", zap.Int32("new", newOut), zap.String("delta", "-1"))
	d.lock.Lock()
	delete(d.attempted, addr)
	if err == nil {
		if oldAddr, ok := d.seeds[addr]; ok {
			newAddr := p.PeerAddr().String()
			d.seeds[addr] = newAddr
			d.log.Info("updating seed address", zap.String("seed", addr),
				zap.String("old", oldAddr), zap.String("new", newAddr))
		}
		d.registerConnected(addr)
	} else {
		d.registerBad(addr, false)
	}
	d.lock.Unlock()
	if err != nil {
		d.log.Info("failed to dial", zap.String("addr", addr), zap.Error(err))
		time.Sleep(d.dialTimeout)
		d.RequestRemote(1)
	}
}
