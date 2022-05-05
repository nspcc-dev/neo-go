package consensus

import (
	"container/list"
	"sync"

	"github.com/nspcc-dev/neo-go/pkg/util"
)

// relayCache is payload cache which is used to store
// last consensus payloads.
type relayCache struct {
	*sync.RWMutex

	maxCap int
	elems  map[util.Uint256]*list.Element
	queue  *list.List
}

// hashable is the type of items which can be stored in the relayCache.
type hashable interface {
	Hash() util.Uint256
}

func newFIFOCache(capacity int) *relayCache {
	return &relayCache{
		RWMutex: new(sync.RWMutex),

		maxCap: capacity,
		elems:  make(map[util.Uint256]*list.Element),
		queue:  list.New(),
	}
}

// Add adds payload into cache if it doesn't already exist there.
func (c *relayCache) Add(p hashable) {
	c.Lock()
	defer c.Unlock()

	h := p.Hash()
	if c.elems[h] != nil {
		return
	}

	if c.queue.Len() >= c.maxCap {
		first := c.queue.Front()
		c.queue.Remove(first)
		delete(c.elems, first.Value.(hashable).Hash())
	}

	e := c.queue.PushBack(p)
	c.elems[h] = e
}

// Has checks if the item is already in cache.
func (c *relayCache) Has(h util.Uint256) bool {
	c.RLock()
	defer c.RUnlock()

	return c.elems[h] != nil
}

// Get returns payload with the specified hash from cache.
func (c *relayCache) Get(h util.Uint256) hashable {
	c.RLock()
	defer c.RUnlock()

	e, ok := c.elems[h]
	if !ok {
		return hashable(nil)
	}
	return e.Value.(hashable)
}
