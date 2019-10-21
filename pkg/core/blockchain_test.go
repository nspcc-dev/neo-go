package core

import (
	"testing"

	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddHeaders(t *testing.T) {
	bc := newTestChain(t)
	h1 := newBlock(1).Header()
	h2 := newBlock(2).Header()
	h3 := newBlock(3).Header()

	if err := bc.AddHeaders(h1, h2, h3); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, h3.Index, bc.HeaderHeight())
	assert.Equal(t, uint32(0), bc.BlockHeight())
	assert.Equal(t, h3.Hash(), bc.CurrentHeaderHash())

	// Add them again, they should not be added.
	if err := bc.AddHeaders(h3, h2, h1); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, h3.Index, bc.HeaderHeight())
	assert.Equal(t, uint32(0), bc.BlockHeight())
	assert.Equal(t, h3.Hash(), bc.CurrentHeaderHash())
}

func TestAddBlock(t *testing.T) {
	bc := newTestChain(t)
	blocks := []*Block{
		newBlock(1, newMinerTX()),
		newBlock(2, newMinerTX()),
		newBlock(3, newMinerTX()),
	}

	for i := 0; i < len(blocks); i++ {
		if err := bc.AddBlock(blocks[i]); err != nil {
			t.Fatal(err)
		}
	}

	lastBlock := blocks[len(blocks)-1]
	assert.Equal(t, lastBlock.Index, bc.HeaderHeight())
	assert.Equal(t, lastBlock.Hash(), bc.CurrentHeaderHash())

	// This one tests persisting blocks, so it does need to persist()
	require.NoError(t, bc.persist())

	for _, block := range blocks {
		key := storage.AppendPrefix(storage.DataBlock, block.Hash().BytesReverse())
		if _, err := bc.store.Get(key); err != nil {
			t.Fatalf("block %s not persisted", block.Hash())
		}
	}

	assert.Equal(t, lastBlock.Index, bc.BlockHeight())
	assert.Equal(t, lastBlock.Hash(), bc.CurrentHeaderHash())
}

func TestGetHeader(t *testing.T) {
	bc := newTestChain(t)
	block := newBlock(1, newMinerTX())
	err := bc.AddBlock(block)
	assert.Nil(t, err)

	// Test unpersisted and persisted access
	for i := 0; i < 2; i++ {
		hash := block.Hash()
		header, err := bc.GetHeader(hash)
		require.NoError(t, err)
		assert.Equal(t, block.Header(), header)

		b2 := newBlock(2)
		_, err = bc.GetHeader(b2.Hash())
		assert.Error(t, err)
		assert.NoError(t, bc.persist())
	}
}

func TestGetBlock(t *testing.T) {
	bc := newTestChain(t)
	blocks := makeBlocks(100)

	for i := 0; i < len(blocks); i++ {
		if err := bc.AddBlock(blocks[i]); err != nil {
			t.Fatal(err)
		}
	}

	// Test unpersisted and persisted access
	for j := 0; j < 2; j++ {
		for i := 0; i < len(blocks); i++ {
			block, err := bc.GetBlock(blocks[i].Hash())
			if err != nil {
				t.Fatalf("can't get block %d: %s, attempt %d", i, err, j)
			}
			assert.Equal(t, blocks[i].Index, block.Index)
			assert.Equal(t, blocks[i].Hash(), block.Hash())
		}
		assert.NoError(t, bc.persist())
	}
}

func TestHasBlock(t *testing.T) {
	bc := newTestChain(t)
	blocks := makeBlocks(50)

	for i := 0; i < len(blocks); i++ {
		if err := bc.AddBlock(blocks[i]); err != nil {
			t.Fatal(err)
		}
	}

	// Test unpersisted and persisted access
	for j := 0; j < 2; j++ {
		for i := 0; i < len(blocks); i++ {
			assert.True(t, bc.HasBlock(blocks[i].Hash()))
		}
		newBlock := newBlock(51)
		assert.False(t, bc.HasBlock(newBlock.Hash()))
		assert.NoError(t, bc.persist())
	}
}

func TestGetTransaction(t *testing.T) {
	b1 := getDecodedBlock(t, 1)
	block := getDecodedBlock(t, 2)
	bc := newTestChain(t)
	// Turn verification off, because these blocks are really from some other chain
	// and can't be verified, but we don't care about that in this test.
	bc.config.VerifyBlocks = false

	assert.Nil(t, bc.AddBlock(b1))
	assert.Nil(t, bc.AddBlock(block))

	// Test unpersisted and persisted access
	for j := 0; j < 2; j++ {
		tx, height, err := bc.GetTransaction(block.Transactions[0].Hash())
		require.Nil(t, err)
		assert.Equal(t, block.Index, height)
		assert.Equal(t, block.Transactions[0], tx)
		assert.Equal(t, 10, io.GetVarSize(tx))
		assert.Equal(t, 1, io.GetVarSize(tx.Attributes))
		assert.Equal(t, 1, io.GetVarSize(tx.Inputs))
		assert.Equal(t, 1, io.GetVarSize(tx.Outputs))
		assert.Equal(t, 1, io.GetVarSize(tx.Scripts))
		assert.NoError(t, bc.persist())
	}
}
