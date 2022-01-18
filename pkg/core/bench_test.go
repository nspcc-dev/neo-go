package core

import (
	"fmt"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/internal/testchain"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/require"
)

func BenchmarkVerifyWitness(t *testing.B) {
	bc := newTestChain(t)
	acc, err := wallet.NewAccount()
	require.NoError(t, err)

	tx := bc.newTestTx(acc.Contract.ScriptHash(), []byte{byte(opcode.PUSH1)})
	require.NoError(t, acc.SignTx(netmode.UnitTestNet, tx))

	t.ResetTimer()
	for n := 0; n < t.N; n++ {
		_, _ = bc.VerifyWitness(tx.Signers[0].Account, tx, &tx.Scripts[0], 100000000)
	}
}

func BenchmarkBlockchain_ForEachNEP17Transfer(t *testing.B) {
	var stores = map[string]func(testing.TB) storage.Store{
		"MemPS": func(t testing.TB) storage.Store {
			return storage.NewMemoryStore()
		},
		"BoltPS":  newBoltStoreForTesting,
		"LevelPS": newLevelDBForTesting,
	}
	startFrom := []int{1, 100, 1000}
	blocksToTake := []int{100, 1000}
	for psName, newPS := range stores {
		for _, startFromBlock := range startFrom {
			for _, nBlocksToTake := range blocksToTake {
				t.Run(fmt.Sprintf("%s_StartFromBlockN-%d_Take%dBlocks", psName, startFromBlock, nBlocksToTake), func(t *testing.B) {
					ps := newPS(t)
					t.Cleanup(func() { ps.Close() })
					benchmarkForEachNEP17Transfer(t, ps, startFromBlock, nBlocksToTake)
				})
			}
		}
	}
}

func benchmarkForEachNEP17Transfer(t *testing.B, ps storage.Store, startFromBlock, nBlocksToTake int) {
	var (
		nonce             uint32 = 1
		chainHeight              = 2_100                            // constant chain height to be able to compare paging results
		transfersPerBlock        = state.TokenTransferBatchSize/4 + // 4 blocks per batch
			state.TokenTransferBatchSize/32 // shift
	)

	bc := newTestChainWithCustomCfgAndStore(t, ps, nil)
	gasHash := bc.contracts.GAS.Hash
	acc := random.Uint160()

	for j := 0; j < chainHeight; j++ {
		w := io.NewBufBinWriter()
		for i := 0; i < transfersPerBlock; i++ {
			emit.AppCall(w.BinWriter, gasHash, "transfer", callflag.All, testchain.MultisigScriptHash(), acc, 1, nil)
			emit.Opcodes(w.BinWriter, opcode.ASSERT)
			require.NoError(t, w.Err)
		}
		script := w.Bytes()
		tx := transaction.New(script, int64(1100_0000*transfersPerBlock))
		tx.ValidUntilBlock = bc.BlockHeight() + 1
		tx.Nonce = nonce
		nonce++
		tx.Signers = []transaction.Signer{{
			Account:          testchain.MultisigScriptHash(),
			Scopes:           transaction.CalledByEntry,
			AllowedContracts: nil,
			AllowedGroups:    nil,
		}}
		require.NoError(t, testchain.SignTx(bc, tx))
		b := bc.newBlock(tx)
		require.NoError(t, bc.AddBlock(b))
		checkTxHalt(t, bc, tx.Hash())
	}

	newestB, err := bc.GetBlock(bc.GetHeaderHash(int(bc.BlockHeight()) - startFromBlock + 1))
	require.NoError(t, err)
	_ = newestB.Timestamp
	oldestB, err := bc.GetBlock(bc.GetHeaderHash(int(newestB.Index) - nBlocksToTake))
	require.NoError(t, err)
	oldestTimestamp := oldestB.Timestamp

	t.ResetTimer()
	t.ReportAllocs()
	t.StartTimer()
	for i := 0; i < t.N; i++ {
		require.NoError(t, bc.ForEachNEP17Transfer(acc, func(t *state.NEP17Transfer) (bool, error) {
			if t.Timestamp < oldestTimestamp {
				// iterating from newest to oldest, already have reached the needed height
				return false, nil
			}
			return true, nil
		}))
	}
	t.StopTimer()
}
