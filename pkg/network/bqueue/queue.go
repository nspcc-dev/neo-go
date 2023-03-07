package bqueue

import (
	"sync"

	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"go.uber.org/atomic"
	"go.uber.org/zap"
)

// Blockqueuer is an interface for a block queue.
type Blockqueuer interface {
	AddBlock(block *block.Block) error
	AddHeaders(...*block.Header) error
	BlockHeight() uint32
}

// Queue is the block queue.
type Queue struct {
	log         *zap.Logger
	queueLock   sync.RWMutex
	queue       []*block.Block
	lastQ       uint32
	checkBlocks chan struct{}
	chain       Blockqueuer
	relayF      func(*block.Block)
	discarded   *atomic.Bool
	len         int
	lenUpdateF  func(int)
}

// CacheSize is the amount of blocks above the current height
// which are stored in the queue.
const CacheSize = 2000

func indexToPosition(i uint32) int {
	return int(i) % CacheSize
}

// New creates an instance of BlockQueue.
func New(bc Blockqueuer, log *zap.Logger, relayer func(*block.Block), lenMetricsUpdater func(l int)) *Queue {
	if log == nil {
		return nil
	}

	return &Queue{
		log:         log,
		queue:       make([]*block.Block, CacheSize),
		checkBlocks: make(chan struct{}, 1),
		chain:       bc,
		relayF:      relayer,
		discarded:   atomic.NewBool(false),
		lenUpdateF:  lenMetricsUpdater,
	}
}

// Run runs the BlockQueue queueing loop. It must be called in a separate routine.
func (bq *Queue) Run() {
	var lastHeight = bq.chain.BlockHeight()
	for {
		_, ok := <-bq.checkBlocks
		if !ok {
			break
		}
		for {
			h := bq.chain.BlockHeight()
			pos := indexToPosition(h + 1)
			bq.queueLock.Lock()
			b := bq.queue[pos]
			// The chain moved forward using blocks from other sources (consensus).
			for i := lastHeight; i < h; i++ {
				old := indexToPosition(i + 1)
				if bq.queue[old] != nil && bq.queue[old].Index == i {
					bq.len--
					bq.queue[old] = nil
				}
			}
			bq.queueLock.Unlock()
			lastHeight = h
			if b == nil {
				break
			}

			err := bq.chain.AddBlock(b)
			if err != nil {
				// The block might already be added by the consensus.
				if bq.chain.BlockHeight() < b.Index {
					bq.log.Warn("blockQueue: failed adding block into the blockchain",
						zap.String("error", err.Error()),
						zap.Uint32("blockHeight", bq.chain.BlockHeight()),
						zap.Uint32("nextIndex", b.Index))
				}
			} else if bq.relayF != nil {
				bq.relayF(b)
			}
			bq.queueLock.Lock()
			bq.len--
			l := bq.len
			if bq.queue[pos] == b {
				bq.queue[pos] = nil
			}
			bq.queueLock.Unlock()
			if bq.lenUpdateF != nil {
				bq.lenUpdateF(l)
			}
		}
	}
}

// PutBlock enqueues block to be added to the chain.
func (bq *Queue) PutBlock(block *block.Block) error {
	h := bq.chain.BlockHeight()
	bq.queueLock.Lock()
	defer bq.queueLock.Unlock()
	if bq.discarded.Load() {
		return nil
	}
	if block.Index <= h || h+CacheSize < block.Index {
		// can easily happen when fetching the same blocks from
		// different peers, thus not considered as error
		return nil
	}
	pos := indexToPosition(block.Index)
	// If we already have it, keep the old block, throw away the new one.
	if bq.queue[pos] == nil || bq.queue[pos].Index < block.Index {
		bq.len++
		bq.queue[pos] = block
		for pos < CacheSize && bq.queue[pos] != nil && bq.lastQ+1 == bq.queue[pos].Index {
			bq.lastQ = bq.queue[pos].Index
			pos++
		}
	}
	// update metrics
	if bq.lenUpdateF != nil {
		bq.lenUpdateF(bq.len)
	}
	select {
	case bq.checkBlocks <- struct{}{}:
		// ok, signalled to goroutine processing queue
	default:
		// it's already busy processing blocks
	}
	return nil
}

// LastQueued returns the index of the last queued block and the queue's capacity
// left.
func (bq *Queue) LastQueued() (uint32, int) {
	bq.queueLock.RLock()
	defer bq.queueLock.RUnlock()
	return bq.lastQ, CacheSize - bq.len
}

// Discard stops the queue and prevents it from accepting more blocks to enqueue.
func (bq *Queue) Discard() {
	if bq.discarded.CAS(false, true) {
		bq.queueLock.Lock()
		close(bq.checkBlocks)
		// Technically we could bq.queue = nil, but this would cost
		// another if in Run().
		for i := 0; i < len(bq.queue); i++ {
			bq.queue[i] = nil
		}
		bq.len = 0
		bq.queueLock.Unlock()
	}
}
