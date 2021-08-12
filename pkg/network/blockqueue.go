package network

import (
	"github.com/Workiva/go-datastructures/queue"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer"
	"go.uber.org/zap"
)

type blockQueue struct {
	log         *zap.Logger
	queue       *queue.PriorityQueue
	checkBlocks chan struct{}
	chain       blockchainer.Blockqueuer
	relayF      func(*block.Block)
}

const (
	// blockCacheSize is the amount of blocks above current height
	// which are stored in queue.
	blockCacheSize = 2000
)

func newBlockQueue(capacity int, bc blockchainer.Blockqueuer, log *zap.Logger, relayer func(*block.Block)) *blockQueue {
	if log == nil {
		return nil
	}

	return &blockQueue{
		log:         log,
		queue:       queue.NewPriorityQueue(capacity, false),
		checkBlocks: make(chan struct{}, 1),
		chain:       bc,
		relayF:      relayer,
	}
}

func (bq *blockQueue) run() {
	for {
		_, ok := <-bq.checkBlocks
		if !ok {
			break
		}
		for {
			item := bq.queue.Peek()
			if item == nil {
				break
			}
			minblock := item.(*block.Block)
			if minblock.Index <= bq.chain.BlockHeight()+1 {
				_, _ = bq.queue.Get(1)
				updateBlockQueueLenMetric(bq.length())
				if minblock.Index == bq.chain.BlockHeight()+1 {
					err := bq.chain.AddBlock(minblock)
					if err != nil {
						// The block might already be added by consensus.
						if bq.chain.BlockHeight() < minblock.Index {
							bq.log.Warn("blockQueue: failed adding block into the blockchain",
								zap.String("error", err.Error()),
								zap.Uint32("blockHeight", bq.chain.BlockHeight()),
								zap.Uint32("nextIndex", minblock.Index))
						}
					} else if bq.relayF != nil {
						bq.relayF(minblock)
					}
				}
			} else {
				break
			}
		}
	}
}

func (bq *blockQueue) putBlock(block *block.Block) error {
	h := bq.chain.BlockHeight()
	if block.Index <= h || h+blockCacheSize < block.Index {
		// can easily happen when fetching the same blocks from
		// different peers, thus not considered as error
		return nil
	}
	err := bq.queue.Put(block)
	// update metrics
	updateBlockQueueLenMetric(bq.length())
	select {
	case bq.checkBlocks <- struct{}{}:
		// ok, signalled to goroutine processing queue
	default:
		// it's already busy processing blocks
	}
	return err
}

func (bq *blockQueue) discard() {
	close(bq.checkBlocks)
	bq.queue.Dispose()
}

func (bq *blockQueue) length() int {
	return bq.queue.Len()
}
