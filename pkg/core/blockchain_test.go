package core

import (
	"errors"
	"fmt"
	"math/big"
	"math/rand"
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/core/mempool"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/internal/testchain"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerifyHeader(t *testing.T) {
	bc := newTestChain(t)
	defer bc.Close()
	prev := bc.topBlock.Load().(*block.Block).Header()
	t.Run("Invalid", func(t *testing.T) {
		t.Run("Hash", func(t *testing.T) {
			h := prev.Hash()
			h[0] = ^h[0]
			hdr := newBlock(bc.config, 1, h).Header()
			require.True(t, errors.Is(bc.verifyHeader(hdr, prev), ErrHdrHashMismatch))
		})
		t.Run("Index", func(t *testing.T) {
			hdr := newBlock(bc.config, 3, prev.Hash()).Header()
			require.True(t, errors.Is(bc.verifyHeader(hdr, prev), ErrHdrIndexMismatch))
		})
		t.Run("Timestamp", func(t *testing.T) {
			hdr := newBlock(bc.config, 1, prev.Hash()).Header()
			hdr.Timestamp = 0
			require.True(t, errors.Is(bc.verifyHeader(hdr, prev), ErrHdrInvalidTimestamp))
		})
	})
	t.Run("Valid", func(t *testing.T) {
		hdr := newBlock(bc.config, 1, prev.Hash()).Header()
		require.NoError(t, bc.verifyHeader(hdr, prev))
	})
}

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

func TestAddBadBlock(t *testing.T) {
	bc := newTestChain(t)
	defer bc.Close()
	// It has ValidUntilBlock == 0, which is wrong
	tx := transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 0)
	tx.Signers = []transaction.Signer{{
		Account: testchain.MultisigScriptHash(),
		Scopes:  transaction.FeeOnly,
	}}
	require.NoError(t, signTx(bc, tx))
	b1 := bc.newBlock(tx)

	require.Error(t, bc.AddBlock(b1))
	bc.config.VerifyTransactions = false
	require.NoError(t, bc.AddBlock(b1))

	b2 := bc.newBlock()
	b2.PrevHash = util.Uint256{}

	require.Error(t, bc.AddBlock(b2))
	bc.config.VerifyBlocks = false
	require.NoError(t, bc.AddBlock(b2))

	tx = transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 0)
	tx.ValidUntilBlock = 128
	tx.Signers = []transaction.Signer{{
		Account: testchain.MultisigScriptHash(),
		Scopes:  transaction.FeeOnly,
	}}
	require.NoError(t, signTx(bc, tx))
	require.NoError(t, bc.PoolTx(tx))
	bc.config.VerifyTransactions = true
	bc.config.VerifyBlocks = true
	b3 := bc.newBlock(tx)
	require.NoError(t, bc.AddBlock(b3))
}

func TestGetHeader(t *testing.T) {
	bc := newTestChain(t)
	tx := transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 0)
	tx.ValidUntilBlock = bc.BlockHeight() + 1
	addSigners(tx)
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

func (bc *Blockchain) newTestTx(h util.Uint160, script []byte) *transaction.Transaction {
	tx := transaction.New(testchain.Network(), script, 1_000_000)
	tx.Nonce = rand.Uint32()
	tx.ValidUntilBlock = 100
	tx.Signers = []transaction.Signer{{
		Account: h,
		Scopes:  transaction.CalledByEntry,
	}}
	tx.NetworkFee = int64(io.GetVarSize(tx)+200 /* witness */) * bc.FeePerByte()
	tx.NetworkFee += 1_000_000 // verification cost
	return tx
}

func TestVerifyTx(t *testing.T) {
	bc := newTestChain(t)
	defer bc.Close()

	accs := make([]*wallet.Account, 2)
	for i := range accs {
		var err error
		accs[i], err = wallet.NewAccount()
		require.NoError(t, err)
	}

	neoHash := bc.contracts.NEO.Hash
	gasHash := bc.contracts.GAS.Hash
	w := io.NewBufBinWriter()
	for _, sc := range []util.Uint160{neoHash, gasHash} {
		for _, a := range accs {
			amount := int64(1_000_000)
			if sc.Equals(gasHash) {
				amount = 1_000_000_000
			}
			emit.AppCallWithOperationAndArgs(w.BinWriter, sc, "transfer",
				neoOwner, a.PrivateKey().GetScriptHash(), amount)
			emit.Opcode(w.BinWriter, opcode.ASSERT)
		}
	}
	require.NoError(t, w.Err)

	txMove := bc.newTestTx(neoOwner, w.Bytes())
	txMove.SystemFee = 1_000_000_000
	require.NoError(t, signTx(bc, txMove))
	b := bc.newBlock(txMove)
	require.NoError(t, bc.AddBlock(b))

	aer, err := bc.GetAppExecResult(txMove.Hash())
	require.NoError(t, err)
	require.Equal(t, aer.VMState, vm.HaltState)

	res, err := invokeNativePolicyMethod(bc, "blockAccount", accs[1].PrivateKey().GetScriptHash().BytesBE())
	require.NoError(t, err)
	checkResult(t, res, stackitem.NewBool(true))

	checkErr := func(t *testing.T, expectedErr error, tx *transaction.Transaction) {
		err := bc.VerifyTx(tx)
		fmt.Println(err)
		require.True(t, errors.Is(err, expectedErr))
	}

	testScript := []byte{byte(opcode.PUSH1)}
	h := accs[0].PrivateKey().GetScriptHash()
	t.Run("Expired", func(t *testing.T) {
		tx := bc.newTestTx(h, testScript)
		tx.ValidUntilBlock = 1
		require.NoError(t, accs[0].SignTx(tx))
		checkErr(t, ErrTxExpired, tx)
	})
	t.Run("BlockedAccount", func(t *testing.T) {
		tx := bc.newTestTx(accs[1].PrivateKey().GetScriptHash(), testScript)
		require.NoError(t, accs[1].SignTx(tx))
		err := bc.VerifyTx(tx)
		require.True(t, errors.Is(err, ErrPolicy))
	})
	t.Run("InsufficientGas", func(t *testing.T) {
		balance := bc.GetUtilityTokenBalance(h)
		tx := bc.newTestTx(h, testScript)
		tx.SystemFee = balance.Int64() + 1
		require.NoError(t, accs[0].SignTx(tx))
		checkErr(t, ErrInsufficientFunds, tx)
	})
	t.Run("TooBigTx", func(t *testing.T) {
		script := make([]byte, transaction.MaxTransactionSize)
		tx := bc.newTestTx(h, script)
		require.NoError(t, accs[0].SignTx(tx))
		checkErr(t, ErrTxTooBig, tx)
	})
	t.Run("SmallNetworkFee", func(t *testing.T) {
		tx := bc.newTestTx(h, testScript)
		tx.NetworkFee = 1
		require.NoError(t, accs[0].SignTx(tx))
		checkErr(t, ErrTxSmallNetworkFee, tx)
	})
	t.Run("Conflict", func(t *testing.T) {
		balance := bc.GetUtilityTokenBalance(h).Int64()
		tx := bc.newTestTx(h, testScript)
		tx.NetworkFee = balance / 2
		require.NoError(t, accs[0].SignTx(tx))
		require.NoError(t, bc.PoolTx(tx))

		tx2 := bc.newTestTx(h, testScript)
		tx2.NetworkFee = balance / 2
		require.NoError(t, accs[0].SignTx(tx2))
		err := bc.PoolTx(tx2)
		require.True(t, errors.Is(err, ErrMemPoolConflict))
	})
	t.Run("NotEnoughWitnesses", func(t *testing.T) {
		tx := bc.newTestTx(h, testScript)
		checkErr(t, ErrTxInvalidWitnessNum, tx)
	})
	t.Run("InvalidWitnessHash", func(t *testing.T) {
		tx := bc.newTestTx(h, testScript)
		require.NoError(t, accs[0].SignTx(tx))
		tx.Scripts[0].VerificationScript = []byte{byte(opcode.PUSHT)}
		checkErr(t, ErrWitnessHashMismatch, tx)
	})
	t.Run("InvalidWitnessSignature", func(t *testing.T) {
		tx := bc.newTestTx(h, testScript)
		require.NoError(t, accs[0].SignTx(tx))
		tx.Scripts[0].InvocationScript[10] = ^tx.Scripts[0].InvocationScript[10]
		checkErr(t, ErrVerificationFailed, tx)
	})
	t.Run("OldTX", func(t *testing.T) {
		tx := bc.newTestTx(h, testScript)
		require.NoError(t, accs[0].SignTx(tx))
		b := bc.newBlock(tx)
		require.NoError(t, bc.AddBlock(b))

		err := bc.VerifyTx(tx)
		require.True(t, errors.Is(err, ErrAlreadyExists))
	})
	t.Run("MemPooledTX", func(t *testing.T) {
		tx := bc.newTestTx(h, testScript)
		require.NoError(t, accs[0].SignTx(tx))
		require.NoError(t, bc.PoolTx(tx))

		err := bc.PoolTx(tx)
		require.True(t, errors.Is(err, ErrAlreadyExists))
	})
	t.Run("MemPoolOOM", func(t *testing.T) {
		bc.memPool = mempool.New(1)
		tx1 := bc.newTestTx(h, testScript)
		tx1.NetworkFee += 10000 // Give it more priority.
		require.NoError(t, accs[0].SignTx(tx1))
		require.NoError(t, bc.PoolTx(tx1))

		tx2 := bc.newTestTx(h, testScript)
		require.NoError(t, accs[0].SignTx(tx2))
		err := bc.PoolTx(tx2)
		require.True(t, errors.Is(err, ErrOOM))
	})
}

func TestVerifyHashAgainstScript(t *testing.T) {
	bc := newTestChain(t)
	defer bc.Close()

	cs, csInvalid := getTestContractState()
	ic := bc.newInteropContext(trigger.Verification, bc.dao, nil, nil)
	require.NoError(t, ic.DAO.PutContractState(cs))
	require.NoError(t, ic.DAO.PutContractState(csInvalid))

	gas := bc.contracts.Policy.GetMaxVerificationGas(ic.DAO)
	t.Run("Contract", func(t *testing.T) {
		t.Run("Missing", func(t *testing.T) {
			newH := cs.ScriptHash()
			newH[0] = ^newH[0]
			w := &transaction.Witness{InvocationScript: []byte{byte(opcode.PUSH4)}}
			err := bc.verifyHashAgainstScript(newH, w, ic, false, gas)
			require.True(t, errors.Is(err, ErrUnknownVerificationContract))
		})
		t.Run("Invalid", func(t *testing.T) {
			w := &transaction.Witness{InvocationScript: []byte{byte(opcode.PUSH4)}}
			err := bc.verifyHashAgainstScript(csInvalid.ScriptHash(), w, ic, false, gas)
			require.True(t, errors.Is(err, ErrInvalidVerificationContract))
		})
		t.Run("ValidSignature", func(t *testing.T) {
			w := &transaction.Witness{InvocationScript: []byte{byte(opcode.PUSH4)}}
			err := bc.verifyHashAgainstScript(cs.ScriptHash(), w, ic, false, gas)
			require.NoError(t, err)
		})
		t.Run("InvalidSignature", func(t *testing.T) {
			w := &transaction.Witness{InvocationScript: []byte{byte(opcode.PUSH3)}}
			err := bc.verifyHashAgainstScript(cs.ScriptHash(), w, ic, false, gas)
			require.True(t, errors.Is(err, ErrVerificationFailed))
		})
	})
	t.Run("NotEnoughGas", func(t *testing.T) {
		verif := []byte{byte(opcode.PUSH1)}
		w := &transaction.Witness{
			InvocationScript:   []byte{byte(opcode.NOP)},
			VerificationScript: verif,
		}
		err := bc.verifyHashAgainstScript(hash.Hash160(verif), w, ic, false, 1)
		require.True(t, errors.Is(err, ErrVerificationFailed))
	})
	t.Run("NoResult", func(t *testing.T) {
		verif := []byte{byte(opcode.DROP)}
		w := &transaction.Witness{
			InvocationScript:   []byte{byte(opcode.PUSH1)},
			VerificationScript: verif,
		}
		err := bc.verifyHashAgainstScript(hash.Hash160(verif), w, ic, false, gas)
		require.True(t, errors.Is(err, ErrVerificationFailed))
	})
	t.Run("TooManyResults", func(t *testing.T) {
		verif := []byte{byte(opcode.NOP)}
		w := &transaction.Witness{
			InvocationScript:   []byte{byte(opcode.PUSH1), byte(opcode.PUSH1)},
			VerificationScript: verif,
		}
		err := bc.verifyHashAgainstScript(hash.Hash160(verif), w, ic, false, gas)
		require.True(t, errors.Is(err, ErrVerificationFailed))
	})
}

func TestMemPoolRemoval(t *testing.T) {
	const added = 16
	const notAdded = 32
	bc := newTestChain(t)
	defer bc.Close()
	addedTxes := make([]*transaction.Transaction, added)
	notAddedTxes := make([]*transaction.Transaction, notAdded)
	for i := range addedTxes {
		addedTxes[i] = bc.newTestTx(testchain.MultisigScriptHash(), []byte{byte(opcode.PUSH1)})
		require.NoError(t, signTx(bc, addedTxes[i]))
		require.NoError(t, bc.PoolTx(addedTxes[i]))
	}
	for i := range notAddedTxes {
		notAddedTxes[i] = bc.newTestTx(testchain.MultisigScriptHash(), []byte{byte(opcode.PUSH1)})
		require.NoError(t, signTx(bc, notAddedTxes[i]))
		require.NoError(t, bc.PoolTx(notAddedTxes[i]))
	}
	b := bc.newBlock(addedTxes...)
	require.NoError(t, bc.AddBlock(b))
	mempool := bc.GetMemPool()
	for _, tx := range addedTxes {
		require.False(t, mempool.ContainsKey(tx.Hash()))
	}
	for _, tx := range notAddedTxes {
		require.True(t, mempool.ContainsKey(tx.Hash()))
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

func TestGetTransaction(t *testing.T) {
	bc := newTestChain(t)
	tx1 := transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH1)}, 0)
	tx1.ValidUntilBlock = 16
	tx1.Signers = []transaction.Signer{{
		Account: testchain.MultisigScriptHash(),
		Scopes:  transaction.CalledByEntry,
	}}
	tx2 := transaction.New(netmode.UnitTestNet, []byte{byte(opcode.PUSH2)}, 0)
	tx2.ValidUntilBlock = 16
	tx2.Signers = []transaction.Signer{{
		Account: testchain.MultisigScriptHash(),
		Scopes:  transaction.CalledByEntry,
	}}
	require.NoError(t, signTx(bc, tx1, tx2))
	b1 := bc.newBlock(tx1)

	assert.Nil(t, bc.AddBlock(b1))
	block := bc.newBlock(tx2)
	txSize := io.GetVarSize(tx2)
	assert.Nil(t, bc.AddBlock(block))

	// Test unpersisted and persisted access
	for j := 0; j < 2; j++ {
		tx, height, err := bc.GetTransaction(block.Transactions[0].Hash())
		require.Nil(t, err)
		assert.Equal(t, block.Index, height)
		assert.Equal(t, block.Transactions[0], tx)
		assert.Equal(t, txSize, io.GetVarSize(tx))
		assert.Equal(t, 1, io.GetVarSize(tx.Attributes))
		assert.Equal(t, 1, io.GetVarSize(tx.Scripts))
		assert.NoError(t, bc.persist())
	}
}

func TestGetClaimable(t *testing.T) {
	bc := newTestChain(t)
	defer bc.Close()

	bc.generationAmount = []int{4, 3, 2, 1}
	bc.decrementInterval = 2
	_, err := bc.genBlocks(10)
	require.NoError(t, err)

	t.Run("first generation period", func(t *testing.T) {
		amount := bc.CalculateClaimable(big.NewInt(1), 0, 2)
		require.EqualValues(t, big.NewInt(8), amount)
	})

	t.Run("a number of full periods", func(t *testing.T) {
		amount := bc.CalculateClaimable(big.NewInt(1), 0, 6)
		require.EqualValues(t, big.NewInt(4+4+3+3+2+2), amount)
	})

	t.Run("start from the 2-nd block", func(t *testing.T) {
		amount := bc.CalculateClaimable(big.NewInt(1), 1, 7)
		require.EqualValues(t, big.NewInt(4+3+3+2+2+1), amount)
	})

	t.Run("end height after generation has ended", func(t *testing.T) {
		amount := bc.CalculateClaimable(big.NewInt(1), 1, 10)
		require.EqualValues(t, big.NewInt(4+3+3+2+2+1+1), amount)
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

func TestSubscriptions(t *testing.T) {
	// We use buffering here as a substitute for reader goroutines, events
	// get queued up and we read them one by one here.
	const chBufSize = 16
	blockCh := make(chan *block.Block, chBufSize)
	txCh := make(chan *transaction.Transaction, chBufSize)
	notificationCh := make(chan *state.NotificationEvent, chBufSize)
	executionCh := make(chan *state.AppExecResult, chBufSize)

	bc := newTestChain(t)
	defer bc.Close()
	bc.SubscribeForBlocks(blockCh)
	bc.SubscribeForTransactions(txCh)
	bc.SubscribeForNotifications(notificationCh)
	bc.SubscribeForExecutions(executionCh)

	assert.Empty(t, notificationCh)
	assert.Empty(t, executionCh)
	assert.Empty(t, blockCh)
	assert.Empty(t, txCh)

	blocks, err := bc.genBlocks(1)
	require.NoError(t, err)
	require.Eventually(t, func() bool { return len(blockCh) != 0 }, time.Second, 10*time.Millisecond)
	assert.Empty(t, notificationCh)
	assert.Len(t, executionCh, 1)
	assert.Empty(t, txCh)

	b := <-blockCh
	assert.Equal(t, blocks[0], b)
	assert.Empty(t, blockCh)

	aer := <-executionCh
	assert.Equal(t, b.Hash(), aer.TxHash)

	script := io.NewBufBinWriter()
	emit.Bytes(script.BinWriter, []byte("yay!"))
	emit.Syscall(script.BinWriter, interopnames.SystemRuntimeNotify)
	require.NoError(t, script.Err)
	txGood1 := transaction.New(netmode.UnitTestNet, script.Bytes(), 0)
	txGood1.Signers = []transaction.Signer{{Account: neoOwner}}
	txGood1.Nonce = 1
	txGood1.ValidUntilBlock = 100500
	require.NoError(t, signTx(bc, txGood1))

	// Reset() reuses the script buffer and we need to keep scripts.
	script = io.NewBufBinWriter()
	emit.Bytes(script.BinWriter, []byte("nay!"))
	emit.Syscall(script.BinWriter, interopnames.SystemRuntimeNotify)
	emit.Opcode(script.BinWriter, opcode.THROW)
	require.NoError(t, script.Err)
	txBad := transaction.New(netmode.UnitTestNet, script.Bytes(), 0)
	txBad.Signers = []transaction.Signer{{Account: neoOwner}}
	txBad.Nonce = 2
	txBad.ValidUntilBlock = 100500
	require.NoError(t, signTx(bc, txBad))

	script = io.NewBufBinWriter()
	emit.Bytes(script.BinWriter, []byte("yay! yay! yay!"))
	emit.Syscall(script.BinWriter, interopnames.SystemRuntimeNotify)
	require.NoError(t, script.Err)
	txGood2 := transaction.New(netmode.UnitTestNet, script.Bytes(), 0)
	txGood2.Signers = []transaction.Signer{{Account: neoOwner}}
	txGood2.Nonce = 3
	txGood2.ValidUntilBlock = 100500
	require.NoError(t, signTx(bc, txGood2))

	invBlock := newBlock(bc.config, bc.BlockHeight()+1, bc.CurrentHeaderHash(), txGood1, txBad, txGood2)
	require.NoError(t, bc.AddBlock(invBlock))

	require.Eventually(t, func() bool {
		return len(blockCh) != 0 && len(txCh) != 0 &&
			len(notificationCh) != 0 && len(executionCh) != 0
	}, time.Second, 10*time.Millisecond)

	b = <-blockCh
	require.Equal(t, invBlock, b)
	assert.Empty(t, blockCh)

	exec := <-executionCh
	require.Equal(t, b.Hash(), exec.TxHash)
	require.Equal(t, exec.VMState, vm.HaltState)

	// 3 burn events for every tx and 1 mint for primary node
	require.True(t, len(notificationCh) >= 4)
	for i := 0; i < 4; i++ {
		notif := <-notificationCh
		require.Equal(t, bc.contracts.GAS.Hash, notif.ScriptHash)
	}

	// Follow in-block transaction order.
	for _, txExpected := range invBlock.Transactions {
		tx := <-txCh
		require.Equal(t, txExpected, tx)
		exec := <-executionCh
		require.Equal(t, tx.Hash(), exec.TxHash)
		if exec.VMState == vm.HaltState {
			notif := <-notificationCh
			require.Equal(t, hash.Hash160(tx.Script), notif.ScriptHash)
		}
	}
	assert.Empty(t, txCh)
	assert.Empty(t, notificationCh)
	assert.Empty(t, executionCh)

	bc.UnsubscribeFromBlocks(blockCh)
	bc.UnsubscribeFromTransactions(txCh)
	bc.UnsubscribeFromNotifications(notificationCh)
	bc.UnsubscribeFromExecutions(executionCh)

	// Ensure that new blocks are processed correctly after unsubscription.
	_, err = bc.genBlocks(2 * chBufSize)
	require.NoError(t, err)
}
