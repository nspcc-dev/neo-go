package bqueue

import (
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/internal/fakechain"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
)

func TestBlockQueue(t *testing.T) {
	chain := fakechain.NewFakeChain()
	// notice, it's not yet running
	bq := New(fakechainBlockQueueAdapter{chain}, zaptest.NewLogger(t), nil, 0, nil, NonBlocking)
	blocks := make([]*block.Block, 11)
	for i := 1; i < 11; i++ {
		blocks[i] = &block.Block{Header: block.Header{Index: uint32(i)}}
	}
	// not the ones expected currently
	for i := 3; i < 5; i++ {
		assert.NoError(t, bq.Put(blocks[i]))
	}
	last, capLeft := bq.LastQueued()
	assert.Equal(t, uint32(0), last)
	assert.Equal(t, DefaultCacheSize-2, capLeft)
	// nothing should be put into the blockchain
	assert.Equal(t, uint32(0), chain.BlockHeight())
	assert.Equal(t, 2, bq.length())
	// now added the expected ones (with duplicates)
	for i := 1; i < 5; i++ {
		assert.NoError(t, bq.Put(blocks[i]))
	}
	// but they're still not put into the blockchain, because bq isn't running
	last, capLeft = bq.LastQueued()
	assert.Equal(t, uint32(4), last)
	assert.Equal(t, DefaultCacheSize-4, capLeft)
	assert.Equal(t, uint32(0), chain.BlockHeight())
	assert.Equal(t, 4, bq.length())
	// block with too big index is dropped
	assert.NoError(t, bq.Put(&block.Block{Header: block.Header{Index: bq.chain.Height() + DefaultCacheSize + 1}}))
	assert.Equal(t, 4, bq.length())
	go bq.Run()
	// run() is asynchronous, so we need some kind of timeout anyway and this is the simplest one
	assert.Eventually(t, func() bool { return chain.BlockHeight() == 4 }, 4*time.Second, 100*time.Millisecond)
	last, capLeft = bq.LastQueued()
	assert.Equal(t, uint32(4), last)
	assert.Equal(t, DefaultCacheSize, capLeft)
	assert.Equal(t, 0, bq.length())
	assert.Equal(t, uint32(4), chain.BlockHeight())
	// put some old blocks
	for i := 1; i < 5; i++ {
		assert.NoError(t, bq.Put(blocks[i]))
	}
	last, capLeft = bq.LastQueued()
	assert.Equal(t, uint32(4), last)
	assert.Equal(t, DefaultCacheSize, capLeft)
	assert.Equal(t, 0, bq.length())
	assert.Equal(t, uint32(4), chain.BlockHeight())
	// unexpected blocks with run() active
	assert.NoError(t, bq.Put(blocks[8]))
	assert.Equal(t, 1, bq.length())
	assert.Equal(t, uint32(4), chain.BlockHeight())
	assert.NoError(t, bq.Put(blocks[7]))
	assert.Equal(t, 2, bq.length())
	assert.Equal(t, uint32(4), chain.BlockHeight())
	// sparse put
	assert.NoError(t, bq.Put(blocks[10]))
	assert.Equal(t, 3, bq.length())
	assert.Equal(t, uint32(4), chain.BlockHeight())
	assert.NoError(t, bq.Put(blocks[6]))
	assert.NoError(t, bq.Put(blocks[5]))
	// run() is asynchronous, so we need some kind of timeout anyway and this is the simplest one
	assert.Eventually(t, func() bool { return chain.BlockHeight() == 8 }, 4*time.Second, 100*time.Millisecond)
	last, capLeft = bq.LastQueued()
	assert.Equal(t, uint32(8), last)
	assert.Equal(t, DefaultCacheSize-1, capLeft)
	assert.Equal(t, 1, bq.length())
	assert.Equal(t, uint32(8), chain.BlockHeight())
	bq.Discard()
	assert.Equal(t, 0, bq.length())
}

// length wraps len access for tests to make them thread-safe.
func (bq *Queue[Q]) length() int {
	bq.queueLock.Lock()
	defer bq.queueLock.Unlock()
	return bq.len
}

type fakechainBlockQueueAdapter struct {
	chain *fakechain.FakeChain
}

func (c fakechainBlockQueueAdapter) AddItem(b *block.Block) error {
	return c.chain.AddBlock(b)
}

func (c fakechainBlockQueueAdapter) AddItems(blk ...*block.Block) error {
	for _, b := range blk {
		if err := c.chain.AddBlock(b); err != nil {
			return err
		}
	}
	return nil
}

func (c fakechainBlockQueueAdapter) Height() uint32 {
	return c.chain.BlockHeight()
}
