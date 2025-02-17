package mpt

import (
	"sync"
	"sync/atomic"

	lru "github.com/hashicorp/golang-lru/v2"
)

type MPTCache struct {
	cache  *lru.Cache[string, Node]
	hits   atomic.Int64
	misses atomic.Int64
	mu     sync.RWMutex
}

func NewMPTCache(size int) (*MPTCache, error) {
	cache, err := lru.New[string, Node](size)
	if err != nil {
		return nil, err
	}
	return &MPTCache{
		cache: cache,
	}, nil
}

func (c *MPTCache) Get(key string) (Node, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if val, ok := c.cache.Get(key); ok {
		c.hits.Add(1)
		return val, true
	}
	c.misses.Add(1)
	return nil, false
}

func (c *MPTCache) Add(key string, value Node) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache.Add(key, value)
}

func (c *MPTCache) Stats() (hits, misses int64) {
	return c.hits.Load(), c.misses.Load()
}
