package extpool

import (
	"container/list"
	"errors"
	"sync"

	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// Pool represents pool of extensible payloads.
type Pool struct {
	lock     sync.RWMutex
	verified map[util.Uint256]*list.Element
	senders  map[util.Uint160]*list.List
	// singleCap represents maximum number of payloads from the single sender.
	singleCap int
	chain     blockchainer.Blockchainer
}

// New returns new payload pool using provided chain.
func New(bc blockchainer.Blockchainer, capacity int) *Pool {
	if capacity <= 0 {
		panic("invalid capacity")
	}

	return &Pool{
		verified:  make(map[util.Uint256]*list.Element),
		senders:   make(map[util.Uint160]*list.List),
		singleCap: capacity,
		chain:     bc,
	}
}

var (
	errDisallowedSender = errors.New("disallowed sender")
	errInvalidHeight    = errors.New("invalid height")
)

// Add adds extensible payload to the pool.
// First return value specifies if payload was new.
// Second one is nil if and only if payload is valid.
func (p *Pool) Add(e *payload.Extensible) (bool, error) {
	if ok, err := p.verify(e); err != nil || !ok {
		return ok, err
	}

	p.lock.Lock()
	defer p.lock.Unlock()

	h := e.Hash()
	if _, ok := p.verified[h]; ok {
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
	return true, nil
}

func (p *Pool) verify(e *payload.Extensible) (bool, error) {
	if _, err := p.chain.VerifyWitness(e.Sender, e, &e.Witness, extensibleVerifyMaxGAS); err != nil {
		return false, err
	}
	h := p.chain.BlockHeight()
	if h < e.ValidBlockStart || e.ValidBlockEnd <= h {
		// We can receive consensus payload for the last or next block
		// which leads to unwanted node disconnect.
		if e.ValidBlockEnd == h {
			return false, nil
		}
		return false, errInvalidHeight
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

const extensibleVerifyMaxGAS = 6000000

// RemoveStale removes invalid payloads after block processing.
func (p *Pool) RemoveStale(index uint32) {
	p.lock.Lock()
	defer p.lock.Unlock()

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
		}
		if lst.Len() == 0 {
			delete(p.senders, s)
		}
	}
}
