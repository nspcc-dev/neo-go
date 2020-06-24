package cache

import (
	"container/list"
	"sync"

	"github.com/nspcc-dev/neo-go/pkg/util"
)

// HashCache is a payload cache which is used to store
// last consensus payloads.
type HashCache struct {
	*sync.RWMutex

	maxCap int
	elems  map[util.Uint256]*list.Element
	queue  *list.List
}

// Hashable is a type of items which can be stored in the HashCache.
type Hashable interface {
	Hash() util.Uint256
}

// NewFIFOCache returns new FIFO cache with the specified capacity.
func NewFIFOCache(capacity int) *HashCache {
	return &HashCache{
		RWMutex: new(sync.RWMutex),

		maxCap: capacity,
		elems:  make(map[util.Uint256]*list.Element),
		queue:  list.New(),
	}
}

// Add adds payload into a cache if it doesn't already exist.
func (c *HashCache) Add(p Hashable) {
	c.Lock()
	defer c.Unlock()

	h := p.Hash()
	if c.elems[h] != nil {
		return
	}

	if c.queue.Len() >= c.maxCap {
		first := c.queue.Front()
		c.queue.Remove(first)
		delete(c.elems, first.Value.(Hashable).Hash())
	}

	e := c.queue.PushBack(p)
	c.elems[h] = e
}

// Has checks if an item is already in cache.
func (c *HashCache) Has(h util.Uint256) bool {
	c.RLock()
	defer c.RUnlock()

	return c.elems[h] != nil
}

// Get returns payload with the specified hash from cache.
func (c *HashCache) Get(h util.Uint256) Hashable {
	c.RLock()
	defer c.RUnlock()

	e, ok := c.elems[h]
	if !ok {
		return Hashable(nil)
	}
	return e.Value.(Hashable)
}
