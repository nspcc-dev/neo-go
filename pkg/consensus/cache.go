package consensus

import (
	"container/list"
	"sync"

	"github.com/CityOfZion/neo-go/pkg/util"
)

// relayCache is a payload cache which is used to store
// last consensus payloads.
type relayCache struct {
	*sync.RWMutex

	maxCap int
	elems  map[util.Uint256]*list.Element
	queue  *list.List
}

func newFIFOCache(capacity int) *relayCache {
	return &relayCache{
		RWMutex: new(sync.RWMutex),

		maxCap: capacity,
		elems:  make(map[util.Uint256]*list.Element),
		queue:  list.New(),
	}
}

// Add adds payload into a cache if it doesn't already exist.
func (c *relayCache) Add(p *Payload) {
	c.Lock()
	defer c.Unlock()

	h := p.Hash()
	if c.elems[h] != nil {
		return
	}

	if c.queue.Len() >= c.maxCap {
		first := c.queue.Front()
		c.queue.Remove(first)
		delete(c.elems, first.Value.(*Payload).Hash())
	}

	e := c.queue.PushBack(p)
	c.elems[h] = e
}

// Get returns payload with the specified hash from cache.
func (c *relayCache) Get(h util.Uint256) *Payload {
	c.RLock()
	defer c.RUnlock()

	e, ok := c.elems[h]
	if !ok {
		return nil
	}
	return e.Value.(*Payload)
}
