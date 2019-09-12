package network

import (
	"time"
)

const (
	maxPoolSize = 200
	connRetries = 3
)

// Discoverer is an interface that is responsible for maintaining
// a healthy connection pool.
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
	unconnectedAddrs map[string]int
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
		unconnectedAddrs: make(map[string]int),
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
	for _, addr := range addrs {
		d.backFill <- addr
	}
}

// PoolCount returns the number of available node addresses.
func (d *DefaultDiscovery) PoolCount() int {
	return len(d.pool)
}

// pushToPoolOrDrop tries to push address given into the pool, but if the pool
// is already full, it just drops it
func (d *DefaultDiscovery) pushToPoolOrDrop(addr string) {
	select {
	case d.pool <- addr:
		// ok, queued
	default:
		// whatever
	}
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
	addrs := make([]string, 0, len(d.unconnectedAddrs))
	for addr := range d.unconnectedAddrs {
		addrs = append(addrs, addr)
	}
	return addrs
}

// BadPeers returns all addresses of bad addrs.
func (d *DefaultDiscovery) BadPeers() []string {
	addrs := make([]string, 0, len(d.badAddrs))
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

func (d *DefaultDiscovery) requestToWork(workCh chan string) {
	var requested int

	for {
		for requested = <-d.requestCh; requested > 0; requested-- {
			select {
			case r := <-d.requestCh:
				if requested < r {
					requested = r
				}
			case addr := <-d.pool:
				workCh <- addr
			}
		}
	}
}

func (d *DefaultDiscovery) run() {
	var (
		maxWorkers = 5
		workCh     = make(chan string)
	)

	for i := 0; i < maxWorkers; i++ {
		go d.work(workCh)
	}

	go d.requestToWork(workCh)
	for {
		select {
		case addr := <-d.backFill:
			if _, ok := d.badAddrs[addr]; ok {
				break
			}
			if _, ok := d.addrs[addr]; !ok {
				d.addrs[addr] = true
				d.unconnectedAddrs[addr] = connRetries
				d.pushToPoolOrDrop(addr)
			}
		case addr := <-d.badAddrCh:
			d.unconnectedAddrs[addr]--
			if d.unconnectedAddrs[addr] > 0 {
				d.pushToPoolOrDrop(addr)
			} else {
				d.badAddrs[addr] = true
				delete(d.unconnectedAddrs, addr)
			}
			d.RequestRemote(1)

		case addr := <-d.connectedCh:
			delete(d.unconnectedAddrs, addr)
		}
	}
}
