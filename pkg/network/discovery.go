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
}

// DefaultDiscovery
type DefaultDiscovery struct {
	transport   Transporter
	dialTimeout time.Duration
	addrs       map[string]bool
	badAddrs    map[string]bool
	requestCh   chan int
	backFill    chan string
	pool        chan string
}

// NewDefaultDiscovery returns a new DefaultDiscovery.
func NewDefaultDiscovery(dt time.Duration, ts Transporter) *DefaultDiscovery {
	d := &DefaultDiscovery{
		transport:   ts,
		dialTimeout: dt,
		addrs:       make(map[string]bool),
		badAddrs:    make(map[string]bool),
		requestCh:   make(chan int),
		backFill:    make(chan string),
		pool:        make(chan string, maxPoolSize),
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

// Request will try to establish a connection with n nodes.
func (d *DefaultDiscovery) RequestRemote(n int) {
	d.requestCh <- n
}

func (d *DefaultDiscovery) work(addrCh, badAddrCh chan string) {
	for {
		addr := <-addrCh
		if err := d.transport.Dial(addr, d.dialTimeout); err != nil {
			badAddrCh <- addr
		}
	}
}

func (d *DefaultDiscovery) next() string {
	return <-d.pool
}

func (d *DefaultDiscovery) run() {
	var (
		maxWorkers = 5
		badAddrCh  = make(chan string)
		workCh     = make(chan string)
	)

	for i := 0; i < maxWorkers; i++ {
		go d.work(workCh, badAddrCh)
	}

	for {
		select {
		case addr := <-d.backFill:
			if _, ok := d.badAddrs[addr]; ok {
				break
			}
			if _, ok := d.addrs[addr]; !ok {
				d.addrs[addr] = true
				d.pool <- addr
			}
		case n := <-d.requestCh:
			go func() {
				for i := 0; i < n; i++ {
					workCh <- d.next()
				}
			}()
		case addr := <-badAddrCh:
			d.badAddrs[addr] = true
			go func() {
				workCh <- d.next()
			}()
		}
	}
}
