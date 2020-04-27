package core

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddHeaders(t *testing.T) {
	bc := newTestChain(t)
	defer bc.Close()
	lastBlock := bc.topBlock.Load().(*block.Block)
	h1 := newBlock(bc.config, 1, lastBlock.Hash()).Header()
	h2 := newBlock(bc.config, 2, h1.Hash()).Header()
	h3 := newBlock(bc.config, 3, h2.Hash()).Header()

	require.NoError(t, bc.AddHeaders())
	require.NoError(t, bc.AddHeaders(h1, h2))
	require.NoError(t, bc.AddHeaders(h2, h3))

	assert.Equal(t, h3.Index, bc.HeaderHeight())
	assert.Equal(t, uint32(0), bc.BlockHeight())
	assert.Equal(t, h3.Hash(), bc.CurrentHeaderHash())

	// Add them again, they should not be added.
	require.NoError(t, bc.AddHeaders(h3, h2, h1))

	assert.Equal(t, h3.Index, bc.HeaderHeight())
	assert.Equal(t, uint32(0), bc.BlockHeight())
	assert.Equal(t, h3.Hash(), bc.CurrentHeaderHash())

	h4 := newBlock(bc.config, 4, h3.Hash().Reverse()).Header()
	h5 := newBlock(bc.config, 5, h4.Hash()).Header()

	assert.Error(t, bc.AddHeaders(h4, h5))
	assert.Equal(t, h3.Index, bc.HeaderHeight())
	assert.Equal(t, uint32(0), bc.BlockHeight())
	assert.Equal(t, h3.Hash(), bc.CurrentHeaderHash())

	h6 := newBlock(bc.config, 4, h3.Hash()).Header()
	h6.Script.InvocationScript = nil
	assert.Error(t, bc.AddHeaders(h6))
	assert.Equal(t, h3.Index, bc.HeaderHeight())
	assert.Equal(t, uint32(0), bc.BlockHeight())
	assert.Equal(t, h3.Hash(), bc.CurrentHeaderHash())
}

func TestAddBlock(t *testing.T) {
	const size = 3
	bc := newTestChain(t)
	blocks, err := bc.genBlocks(size)
	require.NoError(t, err)

	lastBlock := blocks[len(blocks)-1]
	assert.Equal(t, lastBlock.Index, bc.HeaderHeight())
	assert.Equal(t, lastBlock.Hash(), bc.CurrentHeaderHash())

	// This one tests persisting blocks, so it does need to persist()
	require.NoError(t, bc.persist())

	for _, block := range blocks {
		key := storage.AppendPrefix(storage.DataBlock, block.Hash().BytesLE())
		_, err := bc.dao.Store.Get(key)
		require.NoErrorf(t, err, "block %s not persisted", block.Hash())
	}

	assert.Equal(t, lastBlock.Index, bc.BlockHeight())
	assert.Equal(t, lastBlock.Hash(), bc.CurrentHeaderHash())
}

func TestScriptFromWitness(t *testing.T) {
	witness := &transaction.Witness{}
	h := util.Uint160{1, 2, 3}

	res, err := ScriptFromWitness(h, witness)
	require.NoError(t, err)
	require.NotNil(t, res)

	witness.VerificationScript = []byte{4, 8, 15, 16, 23, 42}
	h = hash.Hash160(witness.VerificationScript)

	res, err = ScriptFromWitness(h, witness)
	require.NoError(t, err)
	require.NotNil(t, res)

	h[0] = ^h[0]
	res, err = ScriptFromWitness(h, witness)
	require.Error(t, err)
	require.Nil(t, res)
}

func TestGetHeader(t *testing.T) {
	bc := newTestChain(t)
	tx := transaction.NewContractTX()
	tx.ValidUntilBlock = bc.BlockHeight() + 1
	assert.Nil(t, addSender(tx))
	assert.Nil(t, signTx(bc, tx))
	block := bc.newBlock(tx)
	err := bc.AddBlock(block)
	assert.Nil(t, err)

	// Test unpersisted and persisted access
	for i := 0; i < 2; i++ {
		hash := block.Hash()
		header, err := bc.GetHeader(hash)
		require.NoError(t, err)
		assert.Equal(t, block.Header(), header)

		b2 := bc.newBlock()
		_, err = bc.GetHeader(b2.Hash())
		assert.Error(t, err)
		assert.NoError(t, bc.persist())
	}
}

func TestGetBlock(t *testing.T) {
	bc := newTestChain(t)
	blocks, err := bc.genBlocks(100)
	require.NoError(t, err)

	// Test unpersisted and persisted access
	for j := 0; j < 2; j++ {
		for i := 0; i < len(blocks); i++ {
			block, err := bc.GetBlock(blocks[i].Hash())
			require.NoErrorf(t, err, "can't get block %d: %s, attempt %d", i, err, j)
			assert.Equal(t, blocks[i].Index, block.Index)
			assert.Equal(t, blocks[i].Hash(), block.Hash())
		}
		assert.NoError(t, bc.persist())
	}
}

func TestHasBlock(t *testing.T) {
	bc := newTestChain(t)
	blocks, err := bc.genBlocks(50)
	require.NoError(t, err)

	// Test unpersisted and persisted access
	for j := 0; j < 2; j++ {
		for i := 0; i < len(blocks); i++ {
			assert.True(t, bc.HasBlock(blocks[i].Hash()))
		}
		newBlock := bc.newBlock()
		assert.False(t, bc.HasBlock(newBlock.Hash()))
		assert.NoError(t, bc.persist())
	}
}

//TODO NEO3.0:Update binary
/*
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
*/
func TestGetClaimable(t *testing.T) {
	bc := newTestChain(t)

	bc.generationAmount = []int{4, 3, 2, 1}
	bc.decrementInterval = 2
	_, err := bc.genBlocks(10)
	require.NoError(t, err)

	t.Run("first generation period", func(t *testing.T) {
		amount, sysfee, err := bc.CalculateClaimable(util.Fixed8FromInt64(1), 0, 2)
		require.NoError(t, err)
		require.EqualValues(t, 8, amount)
		require.EqualValues(t, 0, sysfee)
	})

	t.Run("a number of full periods", func(t *testing.T) {
		amount, sysfee, err := bc.CalculateClaimable(util.Fixed8FromInt64(1), 0, 6)
		require.NoError(t, err)
		require.EqualValues(t, 4+4+3+3+2+2, amount)
		require.EqualValues(t, 0, sysfee)
	})

	t.Run("start from the 2-nd block", func(t *testing.T) {
		amount, sysfee, err := bc.CalculateClaimable(util.Fixed8FromInt64(1), 1, 7)
		require.NoError(t, err)
		require.EqualValues(t, 4+3+3+2+2+1, amount)
		require.EqualValues(t, 0, sysfee)
	})

	t.Run("end height after generation has ended", func(t *testing.T) {
		amount, sysfee, err := bc.CalculateClaimable(util.Fixed8FromInt64(1), 1, 10)
		require.NoError(t, err)
		require.EqualValues(t, 4+3+3+2+2+1+1, amount)
		require.EqualValues(t, 0, sysfee)
	})
}

func TestClose(t *testing.T) {
	defer func() {
		r := recover()
		assert.NotNil(t, r)
	}()
	bc := newTestChain(t)
	_, err := bc.genBlocks(10)
	require.NoError(t, err)
	bc.Close()
	// It's a hack, but we use internal knowledge of MemoryStore
	// implementation which makes it completely unusable (up to panicing)
	// after Close().
	_ = bc.dao.Store.Put([]byte{0}, []byte{1})

	// This should never be executed.
	assert.Nil(t, t)
}
