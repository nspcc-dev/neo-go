package extpool

import (
	"container/list"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// Ledger is enough of Blockchain to satisfy Pool.
type Ledger interface {
	BlockHeight() uint32
	GetConfig() config.Blockchain
	GetMillisecondsPerBlock() uint32
	IsExtensibleAllowed(util.Uint160) bool
	VerifyWitness(util.Uint160, hash.Hashable, *transaction.Witness, int64) (int64, error)
}

// Pool represents a pool of extensible payloads.
type Pool struct {
	lock     sync.RWMutex
	verified map[util.Uint256]*list.Element
	senders  map[util.Uint160]*list.List
	// singleCap represents the maximum number of payloads from a single sender.
	singleCap int
	chain     Ledger

	// a set of structures managing stale consensus payloads resend schedule.
	resendDispatcherOn       atomic.Bool
	resendTimerResetCh       chan bool
	resendFunc               func([]util.Uint256)
	resendDispatcherToExitCh chan struct{}
	stopCh                   chan struct{}
}

// New returns a new payload pool using the provided chain.
func New(bc Ledger, capacity int, resendFunc func([]util.Uint256)) *Pool {
	if capacity <= 0 {
		panic("invalid capacity")
	}

	return &Pool{
		verified:                 make(map[util.Uint256]*list.Element),
		senders:                  make(map[util.Uint160]*list.List),
		singleCap:                capacity,
		chain:                    bc,
		resendFunc:               resendFunc,
		resendTimerResetCh:       make(chan bool, 1), // tiny buffer since non-blocking sending is used.
		resendDispatcherToExitCh: make(chan struct{}),
		stopCh:                   make(chan struct{}),
	}
}

// Start starts the Pool's payload resend dispatcher. Caller should wait for
// Start to finish for normal operation.
func (p *Pool) Start() {
	if p.chain.GetConfig().GetNumOfCNs(p.chain.BlockHeight()) == 1 {
		return
	}
	if p.resendDispatcherOn.CompareAndSwap(false, true) {
		go p.resendDispatcher()
	}
}

// Stop stops Pool's payload resend dispatcher and should be called only after
// it's guaranteed there will be no more calls to Add or RemoveStale.
func (p *Pool) Stop() {
	if p.resendDispatcherOn.CompareAndSwap(true, false) {
		close(p.stopCh)
		<-p.resendDispatcherToExitCh
	}
}

var (
	errDisallowedSender = errors.New("disallowed sender")
	// ErrInvalidHeight is returned when Extensible message height is above the
	// current chain's height.
	ErrInvalidHeight = errors.New("invalid height")
)

// Add adds an extensible payload to the pool.
// First return value specifies if the payload was new.
// Second one is nil if and only if the payload is valid.
// Add must not be called once the Pool is stopped.
func (p *Pool) Add(e *payload.Extensible) (bool, error) {
	if ok, err := p.verify(e); err != nil || !ok {
		return ok, err
	}

	p.lock.Lock()

	h := e.Hash()
	if _, ok := p.verified[h]; ok {
		p.lock.Unlock()
		return false, nil
	}

	lst, ok := p.senders[e.Sender]
	if ok && lst.Len() >= p.singleCap {
		value := lst.Remove(lst.Front())
		delete(p.verified, value.(*payload.Extensible).Hash())
	} else if !ok {
		lst = list.New()
		p.senders[e.Sender] = lst
	}

	p.verified[h] = lst.PushBack(e)
	p.lock.Unlock()

	if e.Category == payload.ConsensusCategory {
		select {
		case p.resendTimerResetCh <- true:
		default:
		}
	}

	return true, nil
}

func (p *Pool) verify(e *payload.Extensible) (bool, error) {
	if _, err := p.chain.VerifyWitness(e.Sender, e, &e.Witness, extensibleVerifyMaxGAS); err != nil {
		return false, err
	}
	h := p.chain.BlockHeight()
	if h < e.ValidBlockStart || e.ValidBlockEnd <= h {
		// We can receive a consensus payload for the last or next block
		// which leads to an unwanted node disconnect.
		if e.ValidBlockEnd == h {
			return false, nil
		}
		return false, ErrInvalidHeight
	}
	if !p.chain.IsExtensibleAllowed(e.Sender) {
		return false, errDisallowedSender
	}
	return true, nil
}

// Get returns payload by hash.
func (p *Pool) Get(h util.Uint256) *payload.Extensible {
	p.lock.RLock()
	defer p.lock.RUnlock()

	elem, ok := p.verified[h]
	if !ok {
		return nil
	}
	return elem.Value.(*payload.Extensible)
}

// GetCategory returns the list of extensible hashes matching the specified category.
func (p *Pool) GetCategory(category string) []util.Uint256 {
	p.lock.RLock()
	defer p.lock.RUnlock()

	var res []util.Uint256
	for h, e := range p.verified {
		if e.Value.(*payload.Extensible).Category == category {
			res = append(res, h)
		}
	}
	return res
}

const extensibleVerifyMaxGAS = 6000000

// RemoveStale removes invalid payloads after block processing. RemoveStale
// must not be called once the Pool is stopped.
func (p *Pool) RemoveStale(index uint32) {
	p.lock.Lock()
	var hasConsensus bool
	for s, lst := range p.senders {
		for elem := lst.Front(); elem != nil; {
			e := elem.Value.(*payload.Extensible)
			h := e.Hash()
			old := elem
			elem = elem.Next()

			if e.ValidBlockEnd <= index || !p.chain.IsExtensibleAllowed(e.Sender) {
				delete(p.verified, h)
				lst.Remove(old)
				continue
			}
			if _, err := p.chain.VerifyWitness(e.Sender, e, &e.Witness, extensibleVerifyMaxGAS); err != nil {
				delete(p.verified, h)
				lst.Remove(old)
				continue
			}
			if e.Category == payload.ConsensusCategory {
				hasConsensus = true
			}
		}
		if lst.Len() == 0 {
			delete(p.senders, s)
		}
	}
	p.lock.Unlock()

	select {
	case p.resendTimerResetCh <- hasConsensus:
	default:
	}
}

func (p *Pool) resendDispatcher() {
	var (
		timer          *time.Timer
		threshold      = max(time.Duration(p.chain.GetMillisecondsPerBlock())*time.Millisecond/2, 20*time.Millisecond)
		thresholdShift int
	)
	timerCh := func() <-chan time.Time {
		if timer == nil {
			return nil
		}
		return timer.C
	}
resendLoop:
	for {
		select {
		case <-p.stopCh:
			if timer != nil {
				timer.Stop()
			}
			break resendLoop
		case needTimer := <-p.resendTimerResetCh:
			if needTimer {
				if timer == nil {
					timer = time.NewTimer(threshold)
				} else {
					timer.Reset(threshold)
				}
			} else {
				timer = nil
			}
			thresholdShift = 0
		case <-timerCh():
			hs := p.GetCategory(payload.ConsensusCategory)
			if len(hs) == 0 {
				continue
			}

			p.resendFunc(hs)
			thresholdShift++
			timer.Reset(threshold << thresholdShift)
		}
	}
drainLoop:
	for {
		select {
		case <-p.resendTimerResetCh:
		default:
			break drainLoop
		}
	}
	close(p.resendTimerResetCh)
	close(p.resendDispatcherToExitCh)
}
