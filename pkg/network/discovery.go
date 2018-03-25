package network

import (
	"time"
)

const (
	maxPoolSize = 200
)

// Discoverer is an interface that is responsible for maintaining
// a healty connection pool.
type Discoverer interface {
	BackFill(...string)
	PoolCount() int
	RequestRemote(int)
	RegisterBadAddr(string)
	UnconnectedPeers() []string
	BadPeers() []string
}

// DefaultDiscovery default implementation of the Discoverer interface.
type DefaultDiscovery struct {
	transport        Transporter
	dialTimeout      time.Duration
	addrs            map[string]bool
	badAddrs         map[string]bool
	unconnectedAddrs map[string]bool
	requestCh        chan int
	connectedCh      chan string
	backFill         chan string
	badAddrCh        chan string
	pool             chan string
}

// NewDefaultDiscovery returns a new DefaultDiscovery.
func NewDefaultDiscovery(dt time.Duration, ts Transporter) *DefaultDiscovery {
	d := &DefaultDiscovery{
		transport:        ts,
		dialTimeout:      dt,
		addrs:            make(map[string]bool),
		badAddrs:         make(map[string]bool),
		unconnectedAddrs: make(map[string]bool),
		requestCh:        make(chan int),
		connectedCh:      make(chan string),
		backFill:         make(chan string),
		badAddrCh:        make(chan string),
		pool:             make(chan string, maxPoolSize),
	}
	go d.run()
	return d
}

// BackFill implements the Discoverer interface and will backfill the
// the pool with the given addresses.
func (d *DefaultDiscovery) BackFill(addrs ...string) {
	if len(d.pool) == maxPoolSize {
		return
	}
	for _, addr := range addrs {
		d.backFill <- addr
	}
}

// PoolCount returns the number of available node addresses.
func (d *DefaultDiscovery) PoolCount() int {
	return len(d.pool)
}

// RequestRemote will try to establish a connection with n nodes.
func (d *DefaultDiscovery) RequestRemote(n int) {
	d.requestCh <- n
}

// RegisterBadAddr registers the given address as a bad address.
func (d *DefaultDiscovery) RegisterBadAddr(addr string) {
	d.badAddrCh <- addr
	d.RequestRemote(1)
}

// UnconnectedPeers returns all addresses of unconnected addrs.
func (d *DefaultDiscovery) UnconnectedPeers() []string {
	var addrs []string
	for addr := range d.unconnectedAddrs {
		addrs = append(addrs, addr)
	}
	return addrs
}

// BadPeers returns all addresses of bad addrs.
func (d *DefaultDiscovery) BadPeers() []string {
	var addrs []string
	for addr := range d.badAddrs {
		addrs = append(addrs, addr)
	}
	return addrs
}

func (d *DefaultDiscovery) work(addrCh chan string) {
	for {
		addr := <-addrCh
		if err := d.transport.Dial(addr, d.dialTimeout); err != nil {
			d.badAddrCh <- addr
		} else {
			d.connectedCh <- addr
		}
	}
}

func (d *DefaultDiscovery) next() string {
	return <-d.pool
}

func (d *DefaultDiscovery) run() {
	var (
		maxWorkers = 5
		workCh     = make(chan string)
	)

	for i := 0; i < maxWorkers; i++ {
		go d.work(workCh)
	}

	for {
		select {
		case addr := <-d.backFill:
			if _, ok := d.badAddrs[addr]; ok {
				break
			}
			if _, ok := d.addrs[addr]; !ok {
				d.addrs[addr] = true
				d.unconnectedAddrs[addr] = true
				d.pool <- addr
			}
		case n := <-d.requestCh:
			go func() {
				for i := 0; i < n; i++ {
					workCh <- d.next()
				}
			}()
		case addr := <-d.badAddrCh:
			d.badAddrs[addr] = true
			delete(d.unconnectedAddrs, addr)
			go func() {
				workCh <- d.next()
			}()

		case addr := <-d.connectedCh:
			delete(d.unconnectedAddrs, addr)
		}
	}
}
