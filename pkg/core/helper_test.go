package core

import (
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/internal/testchain"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// newTestChain should be called before newBlock invocation to properly setup
// global state.
func newTestChain(t testing.TB) *Blockchain {
	return newTestChainWithCustomCfg(t, nil)
}

func newTestChainWithCustomCfg(t testing.TB, f func(*config.Config)) *Blockchain {
	return newTestChainWithCustomCfgAndStore(t, nil, f)
}

func newTestChainWithCustomCfgAndStore(t testing.TB, st storage.Store, f func(*config.Config)) *Blockchain {
	chain := initTestChain(t, st, f)
	go chain.Run()
	t.Cleanup(chain.Close)
	return chain
}

func initTestChain(t testing.TB, st storage.Store, f func(*config.Config)) *Blockchain {
	chain, err := initTestChainNoCheck(t, st, f)
	require.NoError(t, err)
	return chain
}

func initTestChainNoCheck(t testing.TB, st storage.Store, f func(*config.Config)) (*Blockchain, error) {
	unitTestNetCfg, err := config.Load("../../config", testchain.Network())
	require.NoError(t, err)
	if f != nil {
		f(&unitTestNetCfg)
	}
	if st == nil {
		st = storage.NewMemoryStore()
	}
	log := zaptest.NewLogger(t)
	if _, ok := t.(*testing.B); ok {
		log = zap.NewNop()
	}
	return NewBlockchain(st, unitTestNetCfg.ProtocolConfiguration, log)
}

func (bc *Blockchain) newBlock(txs ...*transaction.Transaction) *block.Block {
	lastBlock, ok := bc.topBlock.Load().(*block.Block)
	if !ok {
		var err error
		lastBlock, err = bc.GetBlock(bc.GetHeaderHash(int(bc.BlockHeight())))
		if err != nil {
			panic(err)
		}
	}
	if bc.config.StateRootInHeader {
		sr, err := bc.GetStateModule().GetStateRoot(bc.BlockHeight())
		if err != nil {
			panic(err)
		}
		return newBlockWithState(bc.config, lastBlock.Index+1, lastBlock.Hash(), &sr.Root, txs...)
	}
	return newBlock(bc.config, lastBlock.Index+1, lastBlock.Hash(), txs...)
}

func newBlock(cfg config.ProtocolConfiguration, index uint32, prev util.Uint256, txs ...*transaction.Transaction) *block.Block {
	return newBlockWithState(cfg, index, prev, nil, txs...)
}

func newBlockCustom(cfg config.ProtocolConfiguration, f func(b *block.Block),
	txs ...*transaction.Transaction) *block.Block {
	validators, _ := validatorsFromConfig(cfg)
	valScript, _ := smartcontract.CreateDefaultMultiSigRedeemScript(validators)
	witness := transaction.Witness{
		VerificationScript: valScript,
	}
	b := &block.Block{
		Header: block.Header{
			NextConsensus: witness.ScriptHash(),
			Script:        witness,
		},
		Transactions: txs,
	}
	f(b)

	b.RebuildMerkleRoot()
	b.Script.InvocationScript = testchain.Sign(b)
	return b
}

func newBlockWithState(cfg config.ProtocolConfiguration, index uint32, prev util.Uint256,
	prevState *util.Uint256, txs ...*transaction.Transaction) *block.Block {
	return newBlockCustom(cfg, func(b *block.Block) {
		b.PrevHash = prev
		b.Timestamp = uint64(time.Now().UTC().Unix())*1000 + uint64(index)
		b.Index = index

		if prevState != nil {
			b.StateRootEnabled = true
			b.PrevStateRoot = *prevState
		}
	}, txs...)
}

func (bc *Blockchain) genBlocks(n int) ([]*block.Block, error) {
	blocks := make([]*block.Block, n)
	lastHash := bc.topBlock.Load().(*block.Block).Hash()
	lastIndex := bc.topBlock.Load().(*block.Block).Index
	for i := 0; i < n; i++ {
		blocks[i] = newBlock(bc.config, uint32(i)+lastIndex+1, lastHash)
		if err := bc.AddBlock(blocks[i]); err != nil {
			return blocks, err
		}
		lastHash = blocks[i].Hash()
	}
	return blocks, nil
}
