package p2p

import (
	"time"

	"github.com/CityOfZion/neo-go/pkg/network/payload"
)

// Discoverer is an interface that is responsible for maintaining
// a healty connection pool.
type Discoverer interface {
	BackFill(addressList *payload.AddressList)
	Healthy() bool
}

// DefaultDiscovery is the default Discoverer.
type DefaultDiscovery struct {
	discoveryConfig
	pool    chan string
	badPool map[string]bool
}

type peerCountFunc func() int

type discoveryConfig struct {
	healthyPoolCount int
	maxPeers         int
	peerCount        peerCountFunc
	transport        Transporter
	dialTimeout      time.Duration
}

// NewDefaultDiscovery returns a new DefaultDiscovery.
func NewDefaultDiscovery(cfg discoveryConfig) *DefaultDiscovery {
	disc := &DefaultDiscovery{
		discoveryConfig: cfg,
		badPool:         make(map[string]bool),
		pool:            make(chan string, cfg.healthyPoolCount+20),
	}
	go disc.run()
	return disc
}

// BackFill implements the Discoverer interface and will backfill the
// the pool with the given addresses.
func (d *DefaultDiscovery) BackFill(addressList *payload.AddressList) {
	for _, addr := range addressList.Addrs {
		d.pool <- addr.Endpoint.String()
	}
}

// Healthy implements the Discover interface and returns true if
// there are enough addresses in the pool to dial.
func (d *DefaultDiscovery) Healthy() bool {
	return len(d.pool) > d.healthyPoolCount
}

func (d *DefaultDiscovery) work(addrCh, badAddrCh chan string) {
	for {
		addr := <-addrCh
		if err := d.transport.Dial(addr, d.dialTimeout); err != nil {
			badAddrCh <- addr
		}
	}
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

	go func() {
		for {
			if d.peerCount() < d.maxPeers {
				select {
				case addr := <-d.pool:
					workCh <- addr
				}
			}
		}
	}()

	for {
		select {
		case addr := <-badAddrCh:
			d.badPool[addr] = true
		}
	}
}
