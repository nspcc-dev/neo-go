package network

import (
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/network/bqueue"
)

var (
	_ = (bqueue.Queuer[*block.Block])(&stateSyncBlockQueueAdapter{})
	_ = (bqueue.Queuer[*block.Header])(&stateSyncHeaderQueueAdapter{})
	_ = (bqueue.Queuer[*block.Block])(&chainBlockQueueAdapter{})
)

// stateSyncBlockQueueAdapter is a wrapper over StateSync module that that
// implements the [bqueue.Queuer] interface for operating with [*block.Block].
type stateSyncBlockQueueAdapter struct {
	stateSync StateSync
}

// AddItem implements the [bqueue.Queuer] interface.
func (s stateSyncBlockQueueAdapter) AddItem(b *block.Block) error {
	return s.stateSync.AddBlock(b)
}

// AddItems implements the [bqueue.Queuer] interface.
func (s stateSyncBlockQueueAdapter) AddItems(blks ...*block.Block) error {
	panic("AddItems is not implemented for *block.Block")
}

// Height implements the [bqueue.Queuer] interface.
func (s stateSyncBlockQueueAdapter) Height() uint32 {
	return s.stateSync.BlockHeight()
}

// stateSyncHeaderQueueAdapter is a wrapper over StateSync module that
// implements the [bqueue.Queuer] interface for operating with [*block.Header].
type stateSyncHeaderQueueAdapter struct {
	stateSync StateSync
}

// AddItem implements the [bqueue.Queuer] interface.
func (s stateSyncHeaderQueueAdapter) AddItem(h *block.Header) error {
	return s.stateSync.AddHeaders(h)
}

// AddItems implements the [bqueue.Queuer] interface.
func (s stateSyncHeaderQueueAdapter) AddItems(h ...*block.Header) error {
	return s.stateSync.AddHeaders(h...)
}

// Height implements the [bqueue.Queuer] interface.
func (s stateSyncHeaderQueueAdapter) Height() uint32 {
	return s.stateSync.HeaderHeight()
}

// chainBlockQueueAdapter is a wrapper over the [Ledger] interface that
// implements the [bqueue.Queuer] interface for operating with [*block.Block].
type chainBlockQueueAdapter struct {
	chain Ledger
}

// AddItem implements the [bqueue.Queuer] interface.
func (c chainBlockQueueAdapter) AddItem(b *block.Block) error {
	return c.chain.AddBlock(b)
}

// AddItems implements the [bqueue.Queuer] interface.
func (c chainBlockQueueAdapter) AddItems(blk ...*block.Block) error {
	panic("AddItems is not implemented for *block.Block")
}

// Height implements the [bqueue.Queuer] interface.
func (c chainBlockQueueAdapter) Height() uint32 {
	return c.chain.BlockHeight()
}
