package core

import (
	"sync"

	"github.com/CityOfZion/neo-go/pkg/util"
)

// Cache is data structure with fixed type key of Uint256, but has a
// generic value. Used for block, tx and header cache types.
type Cache struct {
	lock sync.RWMutex
	m    map[util.Uint256]interface{}
}

// NewCache returns a ready to use Cache object.
func NewCache() *Cache {
	return &Cache{
		m: make(map[util.Uint256]interface{}),
	}
}

// GetBlock will return a Block type from the cache.
func (c *Cache) GetBlock(h util.Uint256) (block *Block, ok bool) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return c.getBlock(h)
}

func (c *Cache) getBlock(h util.Uint256) (block *Block, ok bool) {
	if v, b := c.m[h]; b {
		block, ok = v.(*Block)
		return
	}
	return
}

// Add adds the given hash along with its value to the cache.
func (c *Cache) Add(h util.Uint256, v interface{}) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.add(h, v)
}

func (c *Cache) add(h util.Uint256, v interface{}) {
	c.m[h] = v
}

func (c *Cache) has(h util.Uint256) bool {
	_, ok := c.m[h]
	return ok
}

// Hash returns whether the cach contains the given hash.
func (c *Cache) Has(h util.Uint256) bool {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.has(h)
}

// Len return the number of items present in the cache.
func (c *Cache) Len() int {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return len(c.m)
}

// Delete removes the item out of the cache.
func (c *Cache) Delete(h util.Uint256) {
	c.lock.Lock()
	defer c.lock.Unlock()
	delete(c.m, h)
}
