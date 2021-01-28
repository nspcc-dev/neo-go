package extpool

import (
	"errors"
	"sync"

	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer"
	"github.com/nspcc-dev/neo-go/pkg/network/payload"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// Pool represents pool of extensible payloads.
type Pool struct {
	lock     sync.RWMutex
	verified map[util.Uint256]*payload.Extensible
	chain    blockchainer.Blockchainer
}

// New returns new payload pool using provided chain.
func New(bc blockchainer.Blockchainer) *Pool {
	return &Pool{
		verified: make(map[util.Uint256]*payload.Extensible),
		chain:    bc,
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
	p.verified[h] = e
	return true, nil
}

func (p *Pool) verify(e *payload.Extensible) (bool, error) {
	if err := p.chain.VerifyWitness(e.Sender, e, &e.Witness, extensibleVerifyMaxGAS); err != nil {
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

	return p.verified[h]
}

const extensibleVerifyMaxGAS = 2000000

// RemoveStale removes invalid payloads after block processing.
func (p *Pool) RemoveStale(index uint32) {
	p.lock.Lock()
	defer p.lock.Unlock()
	for h, e := range p.verified {
		if e.ValidBlockEnd <= index || !p.chain.IsExtensibleAllowed(e.Sender) {
			delete(p.verified, h)
			continue
		}
		if err := p.chain.VerifyWitness(e.Sender, e, &e.Witness, extensibleVerifyMaxGAS); err != nil {
			delete(p.verified, h)
		}
	}
}
