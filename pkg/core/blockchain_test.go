package core

import (
	"testing"

	"github.com/CityOfZion/neo-go/config"
	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
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

	if err := bc.persist(); err != nil {
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
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, block.Header(), header)

	block = newBlock(2)
	hash = block.Hash()
	_, err = bc.GetHeader(block.Hash())
	assert.NotNil(t, err)
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
	assert.Nil(t, bc.persist())

	for i := 0; i < len(blocks); i++ {
		assert.True(t, bc.HasBlock(blocks[i].Hash()))
	}

	newBlock := newBlock(51)
	assert.False(t, bc.HasBlock(newBlock.Hash()))
}

func TestGetTransaction(t *testing.T) {
	block := getDecodedBlock(t, 1)
	bc := newTestChain(t)

	assert.Nil(t, bc.AddBlock(block))
	assert.Nil(t, bc.persistBlock(block))

	tx, height, err := bc.GetTransaction(block.Transactions[0].Hash())
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, block.Index, height)
	assert.Equal(t, block.Transactions[0], tx)
}

func newTestChain(t *testing.T) *Blockchain {
	cfg, err := config.Load("../../config", config.ModePrivNet)
	if err != nil {
		t.Fatal(err)
	}
	chain, err := NewBlockchain(storage.NewMemoryStore(), cfg.ProtocolConfiguration)
	if err != nil {
		t.Fatal(err)
	}
	return chain
}

func TestSize(t *testing.T) {
	txID := "f999c36145a41306c846ea80290416143e8e856559818065be3f4e143c60e43a"
	tx := getTestTransaction(txID, t)

	assert.Equal(t, 283, tx.Size())
	assert.Equal(t, 22, util.GetVarSize(tx.Attributes))
	assert.Equal(t, 35, util.GetVarSize(tx.Inputs))
	assert.Equal(t, 121, util.GetVarSize(tx.Outputs))
	assert.Equal(t, 103, util.GetVarSize(tx.Scripts))
}

func getTestBlockchain(t *testing.T) *Blockchain {
	net := config.ModeUnitTestNet
	configPath := "../../config"
	cfg, err := config.Load(configPath, net)
	if err != nil {
		t.Fatal("could not create levelDB chain", err)
	}

	// adjust datadirectory to point to the correct folder
	cfg.ApplicationConfiguration.DataDirectoryPath = "../rpc/chains/unit_testnet"
	chain, err := NewBlockchainLevelDB(cfg)
	if err != nil {
		t.Fatal("could not create levelDB chain", err)
	}

	return chain
}

func getTestTransaction(txID string, t *testing.T) *transaction.Transaction {
	chain := getTestBlockchain(t)

	txHash, err := util.Uint256DecodeString(txID)
	if err != nil {
		t.Fatalf("could not decode string %s to Uint256: err =%s", txID, err)
	}

	tx, _, err := chain.GetTransaction(txHash)
	if err != nil {
		t.Fatalf("Could not get transaction with hash=%s: err=%s", txHash, err)
	}
	return tx
}
