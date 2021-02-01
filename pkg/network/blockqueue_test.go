package network

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
	bq := newBlockQueue(0, chain, zaptest.NewLogger(t), nil)
	blocks := make([]*block.Block, 11)
	for i := 1; i < 11; i++ {
		blocks[i] = &block.Block{Base: block.Base{Index: uint32(i)}}
	}
	// not the ones expected currently
	for i := 3; i < 5; i++ {
		assert.NoError(t, bq.putBlock(blocks[i]))
	}
	// nothing should be put into the blockchain
	assert.Equal(t, uint32(0), chain.BlockHeight())
	assert.Equal(t, 2, bq.length())
	// now added expected ones (with duplicates)
	for i := 1; i < 5; i++ {
		assert.NoError(t, bq.putBlock(blocks[i]))
	}
	// but they're still not put into the blockchain, because bq isn't running
	assert.Equal(t, uint32(0), chain.BlockHeight())
	assert.Equal(t, 4, bq.length())
	// block with too big index is dropped
	assert.NoError(t, bq.putBlock(&block.Block{Base: block.Base{Index: bq.chain.BlockHeight() + blockCacheSize + 1}}))
	assert.Equal(t, 4, bq.length())
	go bq.run()
	// run() is asynchronous, so we need some kind of timeout anyway and this is the simplest one
	for i := 0; i < 5; i++ {
		if chain.BlockHeight() != 4 {
			time.Sleep(time.Second)
		}
	}
	assert.Equal(t, 0, bq.length())
	assert.Equal(t, uint32(4), chain.BlockHeight())
	// put some old blocks
	for i := 1; i < 5; i++ {
		assert.NoError(t, bq.putBlock(blocks[i]))
	}
	assert.Equal(t, 0, bq.length())
	assert.Equal(t, uint32(4), chain.BlockHeight())
	// unexpected blocks with run() active
	assert.NoError(t, bq.putBlock(blocks[8]))
	assert.Equal(t, 1, bq.length())
	assert.Equal(t, uint32(4), chain.BlockHeight())
	assert.NoError(t, bq.putBlock(blocks[7]))
	assert.Equal(t, 2, bq.length())
	assert.Equal(t, uint32(4), chain.BlockHeight())
	// sparse put
	assert.NoError(t, bq.putBlock(blocks[10]))
	assert.Equal(t, 3, bq.length())
	assert.Equal(t, uint32(4), chain.BlockHeight())
	assert.NoError(t, bq.putBlock(blocks[6]))
	assert.NoError(t, bq.putBlock(blocks[5]))
	// run() is asynchronous, so we need some kind of timeout anyway and this is the simplest one
	for i := 0; i < 5; i++ {
		if chain.BlockHeight() != 8 {
			time.Sleep(time.Second)
		}
	}
	assert.Equal(t, 1, bq.length())
	assert.Equal(t, uint32(8), chain.BlockHeight())
	bq.discard()
	assert.Equal(t, 0, bq.length())
}
