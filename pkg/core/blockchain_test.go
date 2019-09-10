package core

import (
	"context"
	"testing"

	"github.com/CityOfZion/neo-go/config"
	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/util"
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

	assert.Equal(t, 0, bc.blockCache.Len())
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
		newBlock(1),
		newBlock(2),
		newBlock(3),
	}

	for i := 0; i < len(blocks); i++ {
		if err := bc.AddBlock(blocks[i]); err != nil {
			t.Fatal(err)
		}
	}

	lastBlock := blocks[len(blocks)-1]
	assert.Equal(t, 3, bc.blockCache.Len())
	assert.Equal(t, lastBlock.Index, bc.HeaderHeight())
	assert.Equal(t, lastBlock.Hash(), bc.CurrentHeaderHash())

	t.Log(bc.blockCache)

	if err := bc.persist(context.Background()); err != nil {
		t.Fatal(err)
	}

	for _, block := range blocks {
		key := storage.AppendPrefix(storage.DataBlock, block.Hash().BytesReverse())
		if _, err := bc.Get(key); err != nil {
			t.Fatalf("block %s not persisted", block.Hash())
		}
	}

	assert.Equal(t, lastBlock.Index, bc.BlockHeight())
	assert.Equal(t, lastBlock.Hash(), bc.CurrentHeaderHash())
	assert.Equal(t, 0, bc.blockCache.Len())
}

func TestGetHeader(t *testing.T) {
	bc := newTestChain(t)
	block := newBlock(1)
	err := bc.AddBlock(block)
	assert.Nil(t, err)

	hash := block.Hash()
	header, err := bc.GetHeader(hash)
	require.NoError(t, err)
	assert.Equal(t, block.Header(), header)

	block = newBlock(2)
	_, err = bc.GetHeader(block.Hash())
	assert.Error(t, err)
}

func TestGetBlock(t *testing.T) {
	bc := newTestChain(t)
	blocks := makeBlocks(100)

	for i := 0; i < len(blocks); i++ {
		if err := bc.AddBlock(blocks[i]); err != nil {
			t.Fatal(err)
		}
	}

	for i := 0; i < len(blocks); i++ {
		block, err := bc.GetBlock(blocks[i].Hash())
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, blocks[i].Index, block.Index)
		assert.Equal(t, blocks[i].Hash(), block.Hash())
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
	assert.Nil(t, bc.persist(context.Background()))

	for i := 0; i < len(blocks); i++ {
		assert.True(t, bc.HasBlock(blocks[i].Hash()))
	}

	newBlock := newBlock(51)
	assert.False(t, bc.HasBlock(newBlock.Hash()))
}

func TestGetTransaction(t *testing.T) {
	block := getDecodedBlock(t, 2)
	bc := newTestChain(t)

	assert.Nil(t, bc.AddBlock(block))
	assert.Nil(t, bc.persistBlock(block))

	tx, height, err := bc.GetTransaction(block.Transactions[0].Hash())
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, block.Index, height)
	assert.Equal(t, block.Transactions[0], tx)
	assert.Equal(t, 10, tx.Size())
	assert.Equal(t, 1, util.GetVarSize(tx.Attributes))
	assert.Equal(t, 1, util.GetVarSize(tx.Inputs))
	assert.Equal(t, 1, util.GetVarSize(tx.Outputs))
	assert.Equal(t, 1, util.GetVarSize(tx.Scripts))
}

func newTestChain(t *testing.T) *Blockchain {
	cfg, err := config.Load("../../config", config.ModeUnitTestNet)
	if err != nil {
		t.Fatal(err)
	}
	chain, err := NewBlockchain(context.Background(), storage.NewMemoryStore(), cfg.ProtocolConfiguration)
	if err != nil {
		t.Fatal(err)
	}
	return chain
}
