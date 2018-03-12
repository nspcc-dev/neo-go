package network

import (
	"time"

	"github.com/CityOfZion/neo-go/pkg/network/payload"
)

const (
	maxPoolSize = 100
)

// Discoverer is an interface that is responsible for maintaining
// a healty connection pool.
type Discoverer interface {
	BackFill(addressList *payload.AddressList)
	PoolCount() int
	Request(int)
}

// DefaultDiscovery
type DefaultDiscovery struct {
	transport   Transporter
	dialTimeout time.Duration
	addrs       map[string]bool
	badAddrs    map[string]bool
	pool        chan string
	requestCh   chan int
	addrCh      chan string
	que         int
}

// NewDefaultDiscovery returns a new DefaultDiscovery.
func NewDefaultDiscovery(dt time.Duration, ts Transporter) *DefaultDiscovery {
	d := &DefaultDiscovery{
		transport:   ts,
		dialTimeout: dt,
		addrs:       make(map[string]bool),
		badAddrs:    make(map[string]bool),
		pool:        make(chan string, maxPoolSize),
		requestCh:   make(chan int),
		addrCh:      make(chan string),
	}
	go d.run()
	return d
}

// BackFill implements the Discoverer interface and will backfill the
// the pool with the given addresses.
func (d *DefaultDiscovery) BackFill(addressList *payload.AddressList) {
	for i := 0; i < len(addressList.Addrs); i++ {
		d.addrCh <- addressList.Addrs[i].Endpoint.String()
	}
}

// PoolCount returns the number of available node addresses.
func (d *DefaultDiscovery) PoolCount() int {
	return len(d.pool)
}

// Request will try to establish a connection with n nodes.
func (d *DefaultDiscovery) Request(n int) {
	if len(d.pool) < n {
		d.que = n
		return
	}
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
		workCh     = make(chan string, 1000)
	)

	// Start the workers that will connect to the remote node
	// via the given transport.
	for i := 0; i < maxWorkers; i++ {
		go d.work(workCh, badAddrCh)
	}

	// Start the loop that will manage the que.
	go d.queLoop()

	for {
		select {
		case addr := <-d.addrCh:
			if _, ok := d.addrs[addr]; !ok {
				d.addrs[addr] = true
				d.pool <- addr
			}
		case n := <-d.requestCh:
			for i := 0; i < n; i++ {
				workCh <- d.next()
			}
		case addr := <-badAddrCh:
			// If we encountered a bad address, take the next.
			d.badAddrs[addr] = true
			workCh <- d.next()
		}
	}
}

func (d *DefaultDiscovery) queLoop() {
	timer := time.NewTimer(3 * time.Second)
	for {
		<-timer.C
		if d.que > 0 {
			d.que -= len(d.pool)
			d.Request(len(d.pool))
		}
		timer.Reset(3 * time.Second)
	}
}
