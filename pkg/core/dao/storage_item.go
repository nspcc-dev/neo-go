package dao

import (
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

type (
	itemState int

	trackedItem struct {
		state.StorageItem
		State itemState
	}

	itemCache struct {
		st   map[util.Uint160]map[string]*trackedItem
		keys map[util.Uint160][]string
	}
)

const (
	getOp itemState = 1 << iota
	delOp
	addOp
	putOp
	flushedState
)

func newItemCache() *itemCache {
	return &itemCache{
		make(map[util.Uint160]map[string]*trackedItem),
		make(map[util.Uint160][]string),
	}
}

func (c *itemCache) put(h util.Uint160, key []byte, op itemState, item *state.StorageItem) {
	m := c.getItems(h)
	m[string(key)] = &trackedItem{
		StorageItem: *item,
		State:       op,
	}
	c.keys[h] = append(c.keys[h], string(key))
	c.st[h] = m
}

func (c *itemCache) getItem(h util.Uint160, key []byte) *trackedItem {
	m := c.getItems(h)
	return m[string(key)]
}

func (c *itemCache) getItems(h util.Uint160) map[string]*trackedItem {
	m, ok := c.st[h]
	if !ok {
		return make(map[string]*trackedItem)
	}
	return m
}
