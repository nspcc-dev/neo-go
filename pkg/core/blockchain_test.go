package core

import (
	"errors"
	"fmt"
	"math/big"
	"math/rand"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/internal/testchain"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer"
	"github.com/nspcc-dev/neo-go/pkg/core/chaindump"
	"github.com/nspcc-dev/neo-go/pkg/core/fee"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/core/mempool"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativeprices"
	"github.com/nspcc-dev/neo-go/pkg/core/native/noderoles"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
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
	prev := bc.topBlock.Load().(*block.Block).Header
	t.Run("Invalid", func(t *testing.T) {
		t.Run("Hash", func(t *testing.T) {
			h := prev.Hash()
			h[0] = ^h[0]
			hdr := newBlock(bc.config, 1, h).Header
			require.True(t, errors.Is(bc.verifyHeader(&hdr, &prev), ErrHdrHashMismatch))
		})
		t.Run("Index", func(t *testing.T) {
			hdr := newBlock(bc.config, 3, prev.Hash()).Header
			require.True(t, errors.Is(bc.verifyHeader(&hdr, &prev), ErrHdrIndexMismatch))
		})
		t.Run("Timestamp", func(t *testing.T) {
			hdr := newBlock(bc.config, 1, prev.Hash()).Header
			hdr.Timestamp = 0
			require.True(t, errors.Is(bc.verifyHeader(&hdr, &prev), ErrHdrInvalidTimestamp))
		})
	})
	t.Run("Valid", func(t *testing.T) {
		hdr := newBlock(bc.config, 1, prev.Hash()).Header
		require.NoError(t, bc.verifyHeader(&hdr, &prev))
	})
}

func TestAddHeaders(t *testing.T) {
	bc := newTestChain(t)
	lastBlock := bc.topBlock.Load().(*block.Block)
	h1 := newBlock(bc.config, 1, lastBlock.Hash()).Header
	h2 := newBlock(bc.config, 2, h1.Hash()).Header
	h3 := newBlock(bc.config, 3, h2.Hash()).Header

	require.NoError(t, bc.AddHeaders())
	require.NoError(t, bc.AddHeaders(&h1, &h2))
	require.NoError(t, bc.AddHeaders(&h2, &h3))

	assert.Equal(t, h3.Index, bc.HeaderHeight())
	assert.Equal(t, uint32(0), bc.BlockHeight())
	assert.Equal(t, h3.Hash(), bc.CurrentHeaderHash())

	// Add them again, they should not be added.
	require.NoError(t, bc.AddHeaders(&h3, &h2, &h1))

	assert.Equal(t, h3.Index, bc.HeaderHeight())
	assert.Equal(t, uint32(0), bc.BlockHeight())
	assert.Equal(t, h3.Hash(), bc.CurrentHeaderHash())

	h4 := newBlock(bc.config, 4, h3.Hash().Reverse()).Header
	h5 := newBlock(bc.config, 5, h4.Hash()).Header

	assert.Error(t, bc.AddHeaders(&h4, &h5))
	assert.Equal(t, h3.Index, bc.HeaderHeight())
	assert.Equal(t, uint32(0), bc.BlockHeight())
	assert.Equal(t, h3.Hash(), bc.CurrentHeaderHash())

	h6 := newBlock(bc.config, 4, h3.Hash()).Header
	h6.Script.InvocationScript = nil
	assert.Error(t, bc.AddHeaders(&h6))
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
		key := storage.AppendPrefix(storage.DataBlock, block.Hash().BytesBE())
		_, err := bc.dao.Store.Get(key)
		require.NoErrorf(t, err, "block %s not persisted", block.Hash())
	}

	assert.Equal(t, lastBlock.Index, bc.BlockHeight())
	assert.Equal(t, lastBlock.Hash(), bc.CurrentHeaderHash())
}

func TestAddBlockStateRoot(t *testing.T) {
	bc := newTestChainWithCustomCfg(t, func(c *config.Config) {
		c.ProtocolConfiguration.StateRootInHeader = true
	})

	sr, err := bc.GetStateModule().GetStateRoot(bc.BlockHeight())
	require.NoError(t, err)

	tx := newNEP17Transfer(bc.contracts.NEO.Hash, neoOwner, util.Uint160{}, 1)
	tx.ValidUntilBlock = bc.BlockHeight() + 1
	addSigners(neoOwner, tx)
	require.NoError(t, testchain.SignTx(bc, tx))

	lastBlock := bc.topBlock.Load().(*block.Block)
	b := newBlock(bc.config, lastBlock.Index+1, lastBlock.Hash(), tx)
	err = bc.AddBlock(b)
	require.True(t, errors.Is(err, ErrHdrStateRootSetting), "got: %v", err)

	u := sr.Root
	u[0] ^= 0xFF
	b = newBlockWithState(bc.config, lastBlock.Index+1, lastBlock.Hash(), &u, tx)
	err = bc.AddBlock(b)
	require.True(t, errors.Is(err, ErrHdrInvalidStateRoot), "got: %v", err)

	b = bc.newBlock(tx)
	require.NoError(t, bc.AddBlock(b))
}

func TestAddBadBlock(t *testing.T) {
	bc := newTestChain(t)
	// It has ValidUntilBlock == 0, which is wrong
	tx := transaction.New([]byte{byte(opcode.PUSH1)}, 0)
	tx.Signers = []transaction.Signer{{
		Account: testchain.MultisigScriptHash(),
		Scopes:  transaction.None,
	}}
	require.NoError(t, testchain.SignTx(bc, tx))
	b1 := bc.newBlock(tx)

	require.Error(t, bc.AddBlock(b1))
	bc.config.VerifyTransactions = false
	require.NoError(t, bc.AddBlock(b1))

	b2 := bc.newBlock()
	b2.PrevHash = util.Uint256{}

	require.Error(t, bc.AddBlock(b2))
	bc.config.VerifyBlocks = false
	require.NoError(t, bc.AddBlock(b2))

	tx = transaction.New([]byte{byte(opcode.PUSH1)}, 0)
	tx.ValidUntilBlock = 128
	tx.Signers = []transaction.Signer{{
		Account: testchain.MultisigScriptHash(),
		Scopes:  transaction.None,
	}}
	require.NoError(t, testchain.SignTx(bc, tx))
	require.NoError(t, bc.PoolTx(tx))
	bc.config.VerifyTransactions = true
	bc.config.VerifyBlocks = true
	b3 := bc.newBlock(tx)
	require.NoError(t, bc.AddBlock(b3))
}

func TestGetHeader(t *testing.T) {
	bc := newTestChain(t)
	tx := transaction.New([]byte{byte(opcode.PUSH1)}, 0)
	tx.ValidUntilBlock = bc.BlockHeight() + 1
	addSigners(neoOwner, tx)
	assert.Nil(t, testchain.SignTx(bc, tx))
	block := bc.newBlock(tx)
	err := bc.AddBlock(block)
	assert.Nil(t, err)

	// Test unpersisted and persisted access
	for i := 0; i < 2; i++ {
		hash := block.Hash()
		header, err := bc.GetHeader(hash)
		require.NoError(t, err)
		assert.Equal(t, &block.Header, header)

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

	t.Run("store only header", func(t *testing.T) {
		t.Run("non-empty block", func(t *testing.T) {
			tx, err := testchain.NewTransferFromOwner(bc, bc.contracts.NEO.Hash,
				random.Uint160(), 1, 1, 1000)
			b := bc.newBlock(tx)
			require.NoError(t, bc.AddHeaders(&b.Header))

			_, err = bc.GetBlock(b.Hash())
			require.Error(t, err)

			_, err = bc.GetHeader(b.Hash())
			require.NoError(t, err)

			require.NoError(t, bc.AddBlock(b))

			_, err = bc.GetBlock(b.Hash())
			require.NoError(t, err)
		})
		t.Run("empty block", func(t *testing.T) {
			b := bc.newBlock()
			require.NoError(t, bc.AddHeaders(&b.Header))

			_, err = bc.GetBlock(b.Hash())
			require.NoError(t, err)
		})
	})
}

func (bc *Blockchain) newTestTx(h util.Uint160, script []byte) *transaction.Transaction {
	tx := transaction.New(script, 1_000_000)
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

	accs := make([]*wallet.Account, 5)
	for i := range accs {
		var err error
		accs[i], err = wallet.NewAccount()
		require.NoError(t, err)
	}

	oracleAcc := accs[2]
	oraclePubs := keys.PublicKeys{oracleAcc.PrivateKey().PublicKey()}
	require.NoError(t, oracleAcc.ConvertMultisig(1, oraclePubs))

	neoHash := bc.contracts.NEO.Hash
	gasHash := bc.contracts.GAS.Hash
	w := io.NewBufBinWriter()
	for _, sc := range []util.Uint160{neoHash, gasHash} {
		for _, a := range accs {
			amount := int64(1_000_000)
			if sc.Equals(gasHash) {
				amount = 1_000_000_000
			}
			emit.AppCall(w.BinWriter, sc, "transfer", callflag.All,
				neoOwner, a.Contract.ScriptHash(), amount, nil)
			emit.Opcodes(w.BinWriter, opcode.ASSERT)
		}
	}
	emit.AppCall(w.BinWriter, gasHash, "transfer", callflag.All,
		neoOwner, testchain.CommitteeScriptHash(), int64(1_000_000_000), nil)
	emit.Opcodes(w.BinWriter, opcode.ASSERT)
	require.NoError(t, w.Err)

	txMove := bc.newTestTx(neoOwner, w.Bytes())
	txMove.SystemFee = 1_000_000_000
	require.NoError(t, testchain.SignTx(bc, txMove))
	b := bc.newBlock(txMove)
	require.NoError(t, bc.AddBlock(b))

	aer, err := bc.GetAppExecResults(txMove.Hash(), trigger.Application)
	require.NoError(t, err)
	require.Equal(t, 1, len(aer))
	require.Equal(t, aer[0].VMState, vm.HaltState)

	res, err := invokeContractMethodGeneric(bc, 100000000, bc.contracts.Policy.Hash, "blockAccount", true, accs[1].PrivateKey().GetScriptHash().BytesBE())
	require.NoError(t, err)
	checkResult(t, res, stackitem.NewBool(true))

	checkErr := func(t *testing.T, expectedErr error, tx *transaction.Transaction) {
		err := bc.VerifyTx(tx)
		require.True(t, errors.Is(err, expectedErr), "expected: %v, got: %v", expectedErr, err)
	}

	testScript := []byte{byte(opcode.PUSH1)}
	h := accs[0].PrivateKey().GetScriptHash()
	t.Run("Expired", func(t *testing.T) {
		tx := bc.newTestTx(h, testScript)
		tx.ValidUntilBlock = 1
		require.NoError(t, accs[0].SignTx(netmode.UnitTestNet, tx))
		checkErr(t, ErrTxExpired, tx)
	})
	t.Run("BlockedAccount", func(t *testing.T) {
		tx := bc.newTestTx(accs[1].PrivateKey().GetScriptHash(), testScript)
		require.NoError(t, accs[1].SignTx(netmode.UnitTestNet, tx))
		err := bc.VerifyTx(tx)
		require.True(t, errors.Is(err, ErrPolicy))
	})
	t.Run("InsufficientGas", func(t *testing.T) {
		balance := bc.GetUtilityTokenBalance(h)
		tx := bc.newTestTx(h, testScript)
		tx.SystemFee = balance.Int64() + 1
		require.NoError(t, accs[0].SignTx(netmode.UnitTestNet, tx))
		checkErr(t, ErrInsufficientFunds, tx)
	})
	t.Run("TooBigTx", func(t *testing.T) {
		script := make([]byte, transaction.MaxTransactionSize)
		tx := bc.newTestTx(h, script)
		require.NoError(t, accs[0].SignTx(netmode.UnitTestNet, tx))
		checkErr(t, ErrTxTooBig, tx)
	})
	t.Run("NetworkFee", func(t *testing.T) {
		t.Run("SmallNetworkFee", func(t *testing.T) {
			tx := bc.newTestTx(h, testScript)
			tx.NetworkFee = 1
			require.NoError(t, accs[0].SignTx(netmode.UnitTestNet, tx))
			checkErr(t, ErrTxSmallNetworkFee, tx)
		})
		t.Run("AlmostEnoughNetworkFee", func(t *testing.T) {
			tx := bc.newTestTx(h, testScript)
			verificationNetFee, calcultedScriptSize := fee.Calculate(bc.GetBaseExecFee(), accs[0].Contract.Script)
			expectedSize := io.GetVarSize(tx) + calcultedScriptSize
			calculatedNetFee := verificationNetFee + int64(expectedSize)*bc.FeePerByte()
			tx.NetworkFee = calculatedNetFee - 1
			require.NoError(t, accs[0].SignTx(netmode.UnitTestNet, tx))
			require.Equal(t, expectedSize, io.GetVarSize(tx))
			checkErr(t, ErrVerificationFailed, tx)
		})
		t.Run("EnoughNetworkFee", func(t *testing.T) {
			tx := bc.newTestTx(h, testScript)
			verificationNetFee, calcultedScriptSize := fee.Calculate(bc.GetBaseExecFee(), accs[0].Contract.Script)
			expectedSize := io.GetVarSize(tx) + calcultedScriptSize
			calculatedNetFee := verificationNetFee + int64(expectedSize)*bc.FeePerByte()
			tx.NetworkFee = calculatedNetFee
			require.NoError(t, accs[0].SignTx(netmode.UnitTestNet, tx))
			require.Equal(t, expectedSize, io.GetVarSize(tx))
			require.NoError(t, bc.VerifyTx(tx))
		})
		t.Run("CalculateNetworkFee, signature script", func(t *testing.T) {
			tx := bc.newTestTx(h, testScript)
			expectedSize := io.GetVarSize(tx)
			verificationNetFee, calculatedScriptSize := fee.Calculate(bc.GetBaseExecFee(), accs[0].Contract.Script)
			expectedSize += calculatedScriptSize
			expectedNetFee := verificationNetFee + int64(expectedSize)*bc.FeePerByte()
			tx.NetworkFee = expectedNetFee
			require.NoError(t, accs[0].SignTx(netmode.UnitTestNet, tx))
			actualSize := io.GetVarSize(tx)
			require.Equal(t, expectedSize, actualSize)
			interopCtx := bc.newInteropContext(trigger.Verification, bc.dao, nil, tx)
			gasConsumed, err := bc.verifyHashAgainstScript(h, &tx.Scripts[0], interopCtx, -1)
			require.NoError(t, err)
			require.Equal(t, verificationNetFee, gasConsumed)
			require.Equal(t, expectedNetFee, bc.FeePerByte()*int64(actualSize)+gasConsumed)
		})
		t.Run("CalculateNetworkFee, multisignature script", func(t *testing.T) {
			multisigAcc := accs[4]
			pKeys := keys.PublicKeys{multisigAcc.PrivateKey().PublicKey()}
			require.NoError(t, multisigAcc.ConvertMultisig(1, pKeys))
			multisigHash := hash.Hash160(multisigAcc.Contract.Script)
			tx := bc.newTestTx(multisigHash, testScript)
			verificationNetFee, calculatedScriptSize := fee.Calculate(bc.GetBaseExecFee(), multisigAcc.Contract.Script)
			expectedSize := io.GetVarSize(tx) + calculatedScriptSize
			expectedNetFee := verificationNetFee + int64(expectedSize)*bc.FeePerByte()
			tx.NetworkFee = expectedNetFee
			require.NoError(t, multisigAcc.SignTx(netmode.UnitTestNet, tx))
			actualSize := io.GetVarSize(tx)
			require.Equal(t, expectedSize, actualSize)
			interopCtx := bc.newInteropContext(trigger.Verification, bc.dao, nil, tx)
			gasConsumed, err := bc.verifyHashAgainstScript(multisigHash, &tx.Scripts[0], interopCtx, -1)
			require.NoError(t, err)
			require.Equal(t, verificationNetFee, gasConsumed)
			require.Equal(t, expectedNetFee, bc.FeePerByte()*int64(actualSize)+gasConsumed)
		})
	})
	t.Run("InvalidTxScript", func(t *testing.T) {
		tx := bc.newTestTx(h, testScript)
		tx.Script = append(tx.Script, 0xff)
		require.NoError(t, accs[0].SignTx(netmode.UnitTestNet, tx))
		checkErr(t, ErrInvalidScript, tx)
	})
	t.Run("InvalidVerificationScript", func(t *testing.T) {
		tx := bc.newTestTx(h, testScript)
		verif := []byte{byte(opcode.JMP), 3, 0xff, byte(opcode.PUSHT)}
		tx.Signers = append(tx.Signers, transaction.Signer{
			Account: hash.Hash160(verif),
			Scopes:  transaction.Global,
		})
		tx.NetworkFee += 1000000
		require.NoError(t, accs[0].SignTx(netmode.UnitTestNet, tx))
		tx.Scripts = append(tx.Scripts, transaction.Witness{
			InvocationScript:   []byte{},
			VerificationScript: verif,
		})
		checkErr(t, ErrInvalidVerification, tx)
	})
	t.Run("InvalidInvocationScript", func(t *testing.T) {
		tx := bc.newTestTx(h, testScript)
		verif := []byte{byte(opcode.PUSHT)}
		tx.Signers = append(tx.Signers, transaction.Signer{
			Account: hash.Hash160(verif),
			Scopes:  transaction.Global,
		})
		tx.NetworkFee += 1000000
		require.NoError(t, accs[0].SignTx(netmode.UnitTestNet, tx))
		tx.Scripts = append(tx.Scripts, transaction.Witness{
			InvocationScript:   []byte{byte(opcode.JMP), 3, 0xff},
			VerificationScript: verif,
		})
		checkErr(t, ErrInvalidInvocation, tx)
	})
	t.Run("Conflict", func(t *testing.T) {
		balance := bc.GetUtilityTokenBalance(h).Int64()
		tx := bc.newTestTx(h, testScript)
		tx.NetworkFee = balance / 2
		require.NoError(t, accs[0].SignTx(netmode.UnitTestNet, tx))
		require.NoError(t, bc.PoolTx(tx))

		tx2 := bc.newTestTx(h, testScript)
		tx2.NetworkFee = balance / 2
		require.NoError(t, accs[0].SignTx(netmode.UnitTestNet, tx2))
		err := bc.PoolTx(tx2)
		require.True(t, errors.Is(err, ErrMemPoolConflict))
	})
	t.Run("InvalidWitnessHash", func(t *testing.T) {
		tx := bc.newTestTx(h, testScript)
		require.NoError(t, accs[0].SignTx(netmode.UnitTestNet, tx))
		tx.Scripts[0].VerificationScript = []byte{byte(opcode.PUSHT)}
		checkErr(t, ErrWitnessHashMismatch, tx)
	})
	t.Run("InvalidWitnessSignature", func(t *testing.T) {
		tx := bc.newTestTx(h, testScript)
		require.NoError(t, accs[0].SignTx(netmode.UnitTestNet, tx))
		tx.Scripts[0].InvocationScript[10] = ^tx.Scripts[0].InvocationScript[10]
		checkErr(t, ErrVerificationFailed, tx)
	})
	t.Run("InsufficientNetworkFeeForSecondWitness", func(t *testing.T) {
		tx := bc.newTestTx(h, testScript)
		tx.Signers = append(tx.Signers, transaction.Signer{
			Account: accs[3].PrivateKey().GetScriptHash(),
			Scopes:  transaction.Global,
		})
		require.NoError(t, accs[0].SignTx(netmode.UnitTestNet, tx))
		require.NoError(t, accs[3].SignTx(netmode.UnitTestNet, tx))
		checkErr(t, ErrVerificationFailed, tx)
	})
	t.Run("OldTX", func(t *testing.T) {
		tx := bc.newTestTx(h, testScript)
		require.NoError(t, accs[0].SignTx(netmode.UnitTestNet, tx))
		b := bc.newBlock(tx)
		require.NoError(t, bc.AddBlock(b))

		err := bc.VerifyTx(tx)
		require.True(t, errors.Is(err, ErrAlreadyExists))
	})
	t.Run("MemPooledTX", func(t *testing.T) {
		tx := bc.newTestTx(h, testScript)
		require.NoError(t, accs[0].SignTx(netmode.UnitTestNet, tx))
		require.NoError(t, bc.PoolTx(tx))

		err := bc.PoolTx(tx)
		require.True(t, errors.Is(err, ErrAlreadyExists))
	})
	t.Run("MemPoolOOM", func(t *testing.T) {
		bc.memPool = mempool.New(1, 0, false)
		tx1 := bc.newTestTx(h, testScript)
		tx1.NetworkFee += 10000 // Give it more priority.
		require.NoError(t, accs[0].SignTx(netmode.UnitTestNet, tx1))
		require.NoError(t, bc.PoolTx(tx1))

		tx2 := bc.newTestTx(h, testScript)
		require.NoError(t, accs[0].SignTx(netmode.UnitTestNet, tx2))
		err := bc.PoolTx(tx2)
		require.True(t, errors.Is(err, ErrOOM))
	})
	t.Run("Attribute", func(t *testing.T) {
		t.Run("InvalidHighPriority", func(t *testing.T) {
			tx := bc.newTestTx(h, testScript)
			tx.Attributes = append(tx.Attributes, transaction.Attribute{Type: transaction.HighPriority})
			require.NoError(t, accs[0].SignTx(netmode.UnitTestNet, tx))
			checkErr(t, ErrInvalidAttribute, tx)
		})
		t.Run("ValidHighPriority", func(t *testing.T) {
			tx := bc.newTestTx(h, testScript)
			tx.Attributes = append(tx.Attributes, transaction.Attribute{Type: transaction.HighPriority})
			tx.NetworkFee += 4_000_000 // multisig check
			tx.Signers = []transaction.Signer{{
				Account: testchain.CommitteeScriptHash(),
				Scopes:  transaction.None,
			}}
			rawScript := testchain.CommitteeVerificationScript()
			require.NoError(t, err)
			size := io.GetVarSize(tx)
			netFee, sizeDelta := fee.Calculate(bc.GetBaseExecFee(), rawScript)
			tx.NetworkFee += netFee
			tx.NetworkFee += int64(size+sizeDelta) * bc.FeePerByte()
			tx.Scripts = []transaction.Witness{{
				InvocationScript:   testchain.SignCommittee(tx),
				VerificationScript: rawScript,
			}}
			require.NoError(t, bc.VerifyTx(tx))
		})
		t.Run("Oracle", func(t *testing.T) {
			orc := bc.contracts.Oracle
			req := &state.OracleRequest{GasForResponse: 1000_0000}
			require.NoError(t, orc.PutRequestInternal(1, req, bc.dao))

			oracleScript, err := smartcontract.CreateMajorityMultiSigRedeemScript(oraclePubs)
			require.NoError(t, err)
			oracleHash := hash.Hash160(oracleScript)

			// We need to create new transaction,
			// because hashes are cached after signing.
			getOracleTx := func(t *testing.T) *transaction.Transaction {
				tx := bc.newTestTx(h, orc.GetOracleResponseScript())
				resp := &transaction.OracleResponse{
					ID:     1,
					Code:   transaction.Success,
					Result: []byte{1, 2, 3},
				}
				tx.Attributes = []transaction.Attribute{{
					Type:  transaction.OracleResponseT,
					Value: resp,
				}}
				tx.NetworkFee += 4_000_000 // multisig check
				tx.SystemFee = int64(req.GasForResponse - uint64(tx.NetworkFee))
				tx.Signers = []transaction.Signer{{
					Account: oracleHash,
					Scopes:  transaction.None,
				}}
				size := io.GetVarSize(tx)
				netFee, sizeDelta := fee.Calculate(bc.GetBaseExecFee(), oracleScript)
				tx.NetworkFee += netFee
				tx.NetworkFee += int64(size+sizeDelta) * bc.FeePerByte()
				return tx
			}

			t.Run("NoOracleNodes", func(t *testing.T) {
				tx := getOracleTx(t)
				require.NoError(t, oracleAcc.SignTx(netmode.UnitTestNet, tx))
				checkErr(t, ErrInvalidAttribute, tx)
			})

			txSetOracle := transaction.New([]byte{byte(opcode.RET)}, 0) // it's a hack, so we don't need a real script
			setSigner(txSetOracle, testchain.CommitteeScriptHash())
			txSetOracle.Scripts = []transaction.Witness{{
				InvocationScript:   testchain.SignCommittee(txSetOracle),
				VerificationScript: testchain.CommitteeVerificationScript(),
			}}
			bl := block.New(bc.config.StateRootInHeader)
			bl.Index = bc.BlockHeight() + 1
			ic := bc.newInteropContext(trigger.All, bc.dao, bl, txSetOracle)
			ic.SpawnVM()
			ic.VM.LoadScript([]byte{byte(opcode.RET)})
			require.NoError(t, bc.contracts.Designate.DesignateAsRole(ic, noderoles.Oracle, oraclePubs))
			_, err = ic.DAO.Persist()
			require.NoError(t, err)

			t.Run("Valid", func(t *testing.T) {
				tx := getOracleTx(t)
				require.NoError(t, oracleAcc.SignTx(netmode.UnitTestNet, tx))
				require.NoError(t, bc.VerifyTx(tx))

				t.Run("NativeVerify", func(t *testing.T) {
					tx.Signers = append(tx.Signers, transaction.Signer{
						Account: bc.contracts.Oracle.Hash,
						Scopes:  transaction.None,
					})
					tx.Scripts = append(tx.Scripts, transaction.Witness{})
					t.Run("NonZeroVerification", func(t *testing.T) {
						w := io.NewBufBinWriter()
						emit.Opcodes(w.BinWriter, opcode.ABORT)
						emit.Bytes(w.BinWriter, util.Uint160{}.BytesBE())
						emit.Int(w.BinWriter, 0)
						emit.String(w.BinWriter, orc.Manifest.Name)
						tx.Scripts[len(tx.Scripts)-1].VerificationScript = w.Bytes()
						err := bc.VerifyTx(tx)
						require.True(t, errors.Is(err, ErrNativeContractWitness), "got: %v", err)
					})
					t.Run("Good", func(t *testing.T) {
						tx.Scripts[len(tx.Scripts)-1].VerificationScript = nil
						require.NoError(t, bc.VerifyTx(tx))
					})
				})
			})
			t.Run("InvalidRequestID", func(t *testing.T) {
				tx := getOracleTx(t)
				tx.Attributes[0].Value.(*transaction.OracleResponse).ID = 2
				require.NoError(t, oracleAcc.SignTx(netmode.UnitTestNet, tx))
				checkErr(t, ErrInvalidAttribute, tx)
			})
			t.Run("InvalidScope", func(t *testing.T) {
				tx := getOracleTx(t)
				tx.Signers[0].Scopes = transaction.Global
				require.NoError(t, oracleAcc.SignTx(netmode.UnitTestNet, tx))
				checkErr(t, ErrInvalidAttribute, tx)
			})
			t.Run("InvalidScript", func(t *testing.T) {
				tx := getOracleTx(t)
				tx.Script = append(tx.Script, byte(opcode.NOP))
				require.NoError(t, oracleAcc.SignTx(netmode.UnitTestNet, tx))
				checkErr(t, ErrInvalidAttribute, tx)
			})
			t.Run("InvalidSigner", func(t *testing.T) {
				tx := getOracleTx(t)
				tx.Signers[0].Account = accs[0].Contract.ScriptHash()
				require.NoError(t, accs[0].SignTx(netmode.UnitTestNet, tx))
				checkErr(t, ErrInvalidAttribute, tx)
			})
			t.Run("SmallFee", func(t *testing.T) {
				tx := getOracleTx(t)
				tx.SystemFee = 0
				require.NoError(t, oracleAcc.SignTx(netmode.UnitTestNet, tx))
				checkErr(t, ErrInvalidAttribute, tx)
			})
		})
		t.Run("NotValidBefore", func(t *testing.T) {
			getNVBTx := func(height uint32) *transaction.Transaction {
				tx := bc.newTestTx(h, testScript)
				tx.Attributes = append(tx.Attributes, transaction.Attribute{Type: transaction.NotValidBeforeT, Value: &transaction.NotValidBefore{Height: height}})
				tx.NetworkFee += 4_000_000 // multisig check
				tx.Signers = []transaction.Signer{{
					Account: testchain.CommitteeScriptHash(),
					Scopes:  transaction.None,
				}}
				rawScript := testchain.CommitteeVerificationScript()
				require.NoError(t, err)
				size := io.GetVarSize(tx)
				netFee, sizeDelta := fee.Calculate(bc.GetBaseExecFee(), rawScript)
				tx.NetworkFee += netFee
				tx.NetworkFee += int64(size+sizeDelta) * bc.FeePerByte()
				tx.Scripts = []transaction.Witness{{
					InvocationScript:   testchain.SignCommittee(tx),
					VerificationScript: rawScript,
				}}
				return tx
			}
			t.Run("Disabled", func(t *testing.T) {
				tx := getNVBTx(bc.blockHeight + 1)
				require.Error(t, bc.VerifyTx(tx))
			})
			t.Run("Enabled", func(t *testing.T) {
				bc.config.P2PSigExtensions = true
				t.Run("NotYetValid", func(t *testing.T) {
					tx := getNVBTx(bc.blockHeight + 1)
					require.True(t, errors.Is(bc.VerifyTx(tx), ErrInvalidAttribute))
				})
				t.Run("positive", func(t *testing.T) {
					tx := getNVBTx(bc.blockHeight)
					require.NoError(t, bc.VerifyTx(tx))
				})
			})
		})
		t.Run("Reserved", func(t *testing.T) {
			getReservedTx := func(attrType transaction.AttrType) *transaction.Transaction {
				tx := bc.newTestTx(h, testScript)
				tx.Attributes = append(tx.Attributes, transaction.Attribute{Type: attrType, Value: &transaction.Reserved{Value: []byte{1, 2, 3}}})
				tx.NetworkFee += 4_000_000 // multisig check
				tx.Signers = []transaction.Signer{{
					Account: testchain.CommitteeScriptHash(),
					Scopes:  transaction.None,
				}}
				rawScript := testchain.CommitteeVerificationScript()
				require.NoError(t, err)
				size := io.GetVarSize(tx)
				netFee, sizeDelta := fee.Calculate(bc.GetBaseExecFee(), rawScript)
				tx.NetworkFee += netFee
				tx.NetworkFee += int64(size+sizeDelta) * bc.FeePerByte()
				tx.Scripts = []transaction.Witness{{
					InvocationScript:   testchain.SignCommittee(tx),
					VerificationScript: rawScript,
				}}
				return tx
			}
			t.Run("Disabled", func(t *testing.T) {
				tx := getReservedTx(transaction.ReservedLowerBound + 3)
				require.Error(t, bc.VerifyTx(tx))
			})
			t.Run("Enabled", func(t *testing.T) {
				bc.config.ReservedAttributes = true
				tx := getReservedTx(transaction.ReservedLowerBound + 3)
				require.NoError(t, bc.VerifyTx(tx))
			})
		})
		t.Run("Conflicts", func(t *testing.T) {
			getConflictsTx := func(hashes ...util.Uint256) *transaction.Transaction {
				tx := bc.newTestTx(h, testScript)
				tx.Attributes = make([]transaction.Attribute, len(hashes))
				for i, h := range hashes {
					tx.Attributes[i] = transaction.Attribute{
						Type: transaction.ConflictsT,
						Value: &transaction.Conflicts{
							Hash: h,
						},
					}
				}
				tx.NetworkFee += 4_000_000 // multisig check
				tx.Signers = []transaction.Signer{{
					Account: testchain.CommitteeScriptHash(),
					Scopes:  transaction.None,
				}}
				rawScript := testchain.CommitteeVerificationScript()
				require.NoError(t, err)
				size := io.GetVarSize(tx)
				netFee, sizeDelta := fee.Calculate(bc.GetBaseExecFee(), rawScript)
				tx.NetworkFee += netFee
				tx.NetworkFee += int64(size+sizeDelta) * bc.FeePerByte()
				tx.Scripts = []transaction.Witness{{
					InvocationScript:   testchain.SignCommittee(tx),
					VerificationScript: rawScript,
				}}
				return tx
			}
			t.Run("disabled", func(t *testing.T) {
				bc.config.P2PSigExtensions = false
				tx := getConflictsTx(util.Uint256{1, 2, 3})
				require.Error(t, bc.VerifyTx(tx))
			})
			t.Run("enabled", func(t *testing.T) {
				bc.config.P2PSigExtensions = true
				t.Run("dummy on-chain conflict", func(t *testing.T) {
					tx := bc.newTestTx(h, testScript)
					require.NoError(t, accs[0].SignTx(netmode.UnitTestNet, tx))
					dummyTx := transaction.NewTrimmedTX(tx.Hash())
					dummyTx.Version = transaction.DummyVersion
					require.NoError(t, bc.dao.StoreAsTransaction(dummyTx, bc.blockHeight, nil))
					require.True(t, errors.Is(bc.VerifyTx(tx), ErrHasConflicts))
				})
				t.Run("attribute on-chain conflict", func(t *testing.T) {
					tx := transaction.New([]byte{byte(opcode.PUSH1)}, 0)
					tx.ValidUntilBlock = 4242
					tx.Signers = []transaction.Signer{{
						Account: testchain.MultisigScriptHash(),
						Scopes:  transaction.None,
					}}
					require.NoError(t, testchain.SignTx(bc, tx))
					b := bc.newBlock(tx)

					require.NoError(t, bc.AddBlock(b))
					txConflict := getConflictsTx(tx.Hash())
					require.Error(t, bc.VerifyTx(txConflict))
				})
				t.Run("positive", func(t *testing.T) {
					tx := getConflictsTx(random.Uint256())
					require.NoError(t, bc.VerifyTx(tx))
				})
			})
		})
		t.Run("NotaryAssisted", func(t *testing.T) {
			notary, err := wallet.NewAccount()
			require.NoError(t, err)
			txSetNotary := transaction.New([]byte{byte(opcode.RET)}, 0)
			setSigner(txSetNotary, testchain.CommitteeScriptHash())
			txSetNotary.Scripts = []transaction.Witness{{
				InvocationScript:   testchain.SignCommittee(txSetNotary),
				VerificationScript: testchain.CommitteeVerificationScript(),
			}}
			bl := block.New(false)
			bl.Index = bc.BlockHeight() + 1
			ic := bc.newInteropContext(trigger.All, bc.dao, bl, txSetNotary)
			ic.SpawnVM()
			ic.VM.LoadScript([]byte{byte(opcode.RET)})
			require.NoError(t, bc.contracts.Designate.DesignateAsRole(ic, noderoles.P2PNotary, keys.PublicKeys{notary.PrivateKey().PublicKey()}))
			_, err = ic.DAO.Persist()
			require.NoError(t, err)
			getNotaryAssistedTx := func(signaturesCount uint8, serviceFee int64) *transaction.Transaction {
				tx := bc.newTestTx(h, testScript)
				tx.Attributes = append(tx.Attributes, transaction.Attribute{Type: transaction.NotaryAssistedT, Value: &transaction.NotaryAssisted{
					NKeys: signaturesCount,
				}})
				tx.NetworkFee += serviceFee // additional fee for NotaryAssisted attribute
				tx.NetworkFee += 4_000_000  // multisig check
				tx.Signers = []transaction.Signer{{
					Account: testchain.CommitteeScriptHash(),
					Scopes:  transaction.None,
				},
					{
						Account: bc.contracts.Notary.Hash,
						Scopes:  transaction.None,
					},
				}
				rawScript := testchain.CommitteeVerificationScript()
				size := io.GetVarSize(tx)
				netFee, sizeDelta := fee.Calculate(bc.GetBaseExecFee(), rawScript)
				tx.NetworkFee += netFee
				tx.NetworkFee += int64(size+sizeDelta) * bc.FeePerByte()
				tx.Scripts = []transaction.Witness{
					{
						InvocationScript:   testchain.SignCommittee(tx),
						VerificationScript: rawScript,
					},
					{
						InvocationScript: append([]byte{byte(opcode.PUSHDATA1), 64}, notary.PrivateKey().SignHashable(uint32(testchain.Network()), tx)...),
					},
				}
				return tx
			}
			t.Run("Disabled", func(t *testing.T) {
				bc.config.P2PSigExtensions = false
				tx := getNotaryAssistedTx(0, 0)
				require.True(t, errors.Is(bc.VerifyTx(tx), ErrInvalidAttribute))
			})
			t.Run("Enabled, insufficient network fee", func(t *testing.T) {
				bc.config.P2PSigExtensions = true
				tx := getNotaryAssistedTx(1, 0)
				require.Error(t, bc.VerifyTx(tx))
			})
			t.Run("Test verify", func(t *testing.T) {
				bc.config.P2PSigExtensions = true
				t.Run("no NotaryAssisted attribute", func(t *testing.T) {
					tx := getNotaryAssistedTx(1, (1+1)*transaction.NotaryServiceFeePerKey)
					tx.Attributes = []transaction.Attribute{}
					tx.Signers = []transaction.Signer{
						{
							Account: testchain.CommitteeScriptHash(),
							Scopes:  transaction.None,
						},
						{
							Account: bc.contracts.Notary.Hash,
							Scopes:  transaction.None,
						},
					}
					tx.Scripts = []transaction.Witness{
						{
							InvocationScript:   testchain.SignCommittee(tx),
							VerificationScript: testchain.CommitteeVerificationScript(),
						},
						{
							InvocationScript: append([]byte{byte(opcode.PUSHDATA1), 64}, notary.PrivateKey().SignHashable(uint32(testchain.Network()), tx)...),
						},
					}
					require.Error(t, bc.VerifyTx(tx))
				})
				t.Run("no deposit", func(t *testing.T) {
					tx := getNotaryAssistedTx(1, (1+1)*transaction.NotaryServiceFeePerKey)
					tx.Signers = []transaction.Signer{
						{
							Account: bc.contracts.Notary.Hash,
							Scopes:  transaction.None,
						},
						{
							Account: testchain.CommitteeScriptHash(),
							Scopes:  transaction.None,
						},
					}
					tx.Scripts = []transaction.Witness{
						{
							InvocationScript: append([]byte{byte(opcode.PUSHDATA1), 64}, notary.PrivateKey().SignHashable(uint32(testchain.Network()), tx)...),
						},
						{
							InvocationScript:   testchain.SignCommittee(tx),
							VerificationScript: testchain.CommitteeVerificationScript(),
						},
					}
					require.Error(t, bc.VerifyTx(tx))
				})
				t.Run("bad Notary signer scope", func(t *testing.T) {
					tx := getNotaryAssistedTx(1, (1+1)*transaction.NotaryServiceFeePerKey)
					tx.Signers = []transaction.Signer{
						{
							Account: testchain.CommitteeScriptHash(),
							Scopes:  transaction.None,
						},
						{
							Account: bc.contracts.Notary.Hash,
							Scopes:  transaction.CalledByEntry,
						},
					}
					tx.Scripts = []transaction.Witness{
						{
							InvocationScript:   testchain.SignCommittee(tx),
							VerificationScript: testchain.CommitteeVerificationScript(),
						},
						{
							InvocationScript: append([]byte{byte(opcode.PUSHDATA1), 64}, notary.PrivateKey().SignHashable(uint32(testchain.Network()), tx)...),
						},
					}
					require.Error(t, bc.VerifyTx(tx))
				})
				t.Run("not signed by Notary", func(t *testing.T) {
					tx := getNotaryAssistedTx(1, (1+1)*transaction.NotaryServiceFeePerKey)
					tx.Signers = []transaction.Signer{
						{
							Account: testchain.CommitteeScriptHash(),
							Scopes:  transaction.None,
						},
					}
					tx.Scripts = []transaction.Witness{
						{
							InvocationScript:   testchain.SignCommittee(tx),
							VerificationScript: testchain.CommitteeVerificationScript(),
						},
					}
					require.Error(t, bc.VerifyTx(tx))
				})
				t.Run("bad Notary node witness", func(t *testing.T) {
					tx := getNotaryAssistedTx(1, (1+1)*transaction.NotaryServiceFeePerKey)
					tx.Signers = []transaction.Signer{
						{
							Account: testchain.CommitteeScriptHash(),
							Scopes:  transaction.None,
						},
						{
							Account: bc.contracts.Notary.Hash,
							Scopes:  transaction.None,
						},
					}
					acc, err := keys.NewPrivateKey()
					require.NoError(t, err)
					tx.Scripts = []transaction.Witness{
						{
							InvocationScript:   testchain.SignCommittee(tx),
							VerificationScript: testchain.CommitteeVerificationScript(),
						},
						{
							InvocationScript: append([]byte{byte(opcode.PUSHDATA1), 64}, acc.SignHashable(uint32(testchain.Network()), tx)...),
						},
					}
					require.Error(t, bc.VerifyTx(tx))
				})
				t.Run("missing payer", func(t *testing.T) {
					tx := getNotaryAssistedTx(1, (1+1)*transaction.NotaryServiceFeePerKey)
					tx.Signers = []transaction.Signer{
						{
							Account: bc.contracts.Notary.Hash,
							Scopes:  transaction.None,
						},
					}
					tx.Scripts = []transaction.Witness{
						{
							InvocationScript: append([]byte{byte(opcode.PUSHDATA1), 64}, notary.PrivateKey().SignHashable(uint32(testchain.Network()), tx)...),
						},
					}
					require.Error(t, bc.VerifyTx(tx))
				})
				t.Run("positive", func(t *testing.T) {
					tx := getNotaryAssistedTx(1, (1+1)*transaction.NotaryServiceFeePerKey)
					require.NoError(t, bc.VerifyTx(tx))
				})
			})
		})
	})
	t.Run("Partially-filled transaction", func(t *testing.T) {
		bc.config.P2PSigExtensions = true
		getPartiallyFilledTx := func(nvb uint32, validUntil uint32) *transaction.Transaction {
			tx := bc.newTestTx(h, testScript)
			tx.ValidUntilBlock = validUntil
			tx.Attributes = []transaction.Attribute{
				{
					Type:  transaction.NotValidBeforeT,
					Value: &transaction.NotValidBefore{Height: nvb},
				},
				{
					Type:  transaction.NotaryAssistedT,
					Value: &transaction.NotaryAssisted{NKeys: 0},
				},
			}
			tx.Signers = []transaction.Signer{
				{
					Account: bc.contracts.Notary.Hash,
					Scopes:  transaction.None,
				},
				{
					Account: testchain.MultisigScriptHash(),
					Scopes:  transaction.None,
				},
			}
			size := io.GetVarSize(tx)
			netFee, sizeDelta := fee.Calculate(bc.GetBaseExecFee(), testchain.MultisigVerificationScript())
			tx.NetworkFee = netFee + // multisig witness verification price
				int64(size)*bc.FeePerByte() + // fee for unsigned size
				int64(sizeDelta)*bc.FeePerByte() + //fee for multisig size
				66*bc.FeePerByte() + // fee for Notary signature size (66 bytes for Invocation script and 0 bytes for Verification script)
				2*bc.FeePerByte() + // fee for the length of each script in Notary witness (they are nil, so we did not take them into account during `size` calculation)
				transaction.NotaryServiceFeePerKey + // fee for Notary attribute
				fee.Opcode(bc.GetBaseExecFee(), // Notary verification script
					opcode.PUSHDATA1, opcode.RET, // invocation script
					opcode.PUSH0, opcode.SYSCALL, opcode.RET) + // Neo.Native.Call
				nativeprices.NotaryVerificationPrice*bc.GetBaseExecFee() // Notary witness verification price
			tx.Scripts = []transaction.Witness{
				{
					InvocationScript:   append([]byte{byte(opcode.PUSHDATA1), 64}, make([]byte, 64, 64)...),
					VerificationScript: []byte{},
				},
				{
					InvocationScript:   testchain.Sign(tx),
					VerificationScript: testchain.MultisigVerificationScript(),
				},
			}
			return tx
		}

		mp := mempool.New(10, 1, false)
		verificationF := func(bc blockchainer.Blockchainer, tx *transaction.Transaction, data interface{}) error {
			if data.(int) > 5 {
				return errors.New("bad data")
			}
			return nil
		}
		t.Run("failed pre-verification", func(t *testing.T) {
			tx := getPartiallyFilledTx(bc.blockHeight, bc.blockHeight+1)
			require.Error(t, bc.PoolTxWithData(tx, 6, mp, bc, verificationF)) // here and below let's use `bc` instead of proper NotaryFeer for the test simplicity.
		})
		t.Run("GasLimitExceeded during witness verification", func(t *testing.T) {
			tx := getPartiallyFilledTx(bc.blockHeight, bc.blockHeight+1)
			tx.NetworkFee-- // to check that NetworkFee was set correctly in getPartiallyFilledTx
			tx.Scripts = []transaction.Witness{
				{
					InvocationScript:   append([]byte{byte(opcode.PUSHDATA1), 64}, make([]byte, 64, 64)...),
					VerificationScript: []byte{},
				},
				{
					InvocationScript:   testchain.Sign(tx),
					VerificationScript: testchain.MultisigVerificationScript(),
				},
			}
			require.Error(t, bc.PoolTxWithData(tx, 5, mp, bc, verificationF))
		})
		t.Run("bad NVB: too big", func(t *testing.T) {
			tx := getPartiallyFilledTx(bc.blockHeight+bc.contracts.Notary.GetMaxNotValidBeforeDelta(bc.dao)+1, bc.blockHeight+1)
			require.True(t, errors.Is(bc.PoolTxWithData(tx, 5, mp, bc, verificationF), ErrInvalidAttribute))
		})
		t.Run("bad ValidUntilBlock: too small", func(t *testing.T) {
			tx := getPartiallyFilledTx(bc.blockHeight, bc.blockHeight+bc.contracts.Notary.GetMaxNotValidBeforeDelta(bc.dao)+1)
			require.True(t, errors.Is(bc.PoolTxWithData(tx, 5, mp, bc, verificationF), ErrInvalidAttribute))
		})
		t.Run("good", func(t *testing.T) {
			tx := getPartiallyFilledTx(bc.blockHeight, bc.blockHeight+1)
			require.NoError(t, bc.PoolTxWithData(tx, 5, mp, bc, verificationF))
		})
	})
}

func TestVerifyHashAgainstScript(t *testing.T) {
	bc := newTestChain(t)

	cs, csInvalid := getTestContractState(bc)
	ic := bc.newInteropContext(trigger.Verification, bc.dao, nil, nil)
	require.NoError(t, bc.contracts.Management.PutContractState(bc.dao, cs))
	require.NoError(t, bc.contracts.Management.PutContractState(bc.dao, csInvalid))

	gas := bc.contracts.Policy.GetMaxVerificationGas(ic.DAO)
	t.Run("Contract", func(t *testing.T) {
		t.Run("Missing", func(t *testing.T) {
			newH := cs.Hash
			newH[0] = ^newH[0]
			w := &transaction.Witness{InvocationScript: []byte{byte(opcode.PUSH4)}}
			_, err := bc.verifyHashAgainstScript(newH, w, ic, gas)
			require.True(t, errors.Is(err, ErrUnknownVerificationContract))
		})
		t.Run("Invalid", func(t *testing.T) {
			w := &transaction.Witness{InvocationScript: []byte{byte(opcode.PUSH4)}}
			_, err := bc.verifyHashAgainstScript(csInvalid.Hash, w, ic, gas)
			require.True(t, errors.Is(err, ErrInvalidVerificationContract))
		})
		t.Run("ValidSignature", func(t *testing.T) {
			w := &transaction.Witness{InvocationScript: []byte{byte(opcode.PUSH4)}}
			_, err := bc.verifyHashAgainstScript(cs.Hash, w, ic, gas)
			require.NoError(t, err)
		})
		t.Run("InvalidSignature", func(t *testing.T) {
			w := &transaction.Witness{InvocationScript: []byte{byte(opcode.PUSH3)}}
			_, err := bc.verifyHashAgainstScript(cs.Hash, w, ic, gas)
			require.True(t, errors.Is(err, ErrVerificationFailed))
		})
	})
	t.Run("NotEnoughGas", func(t *testing.T) {
		verif := []byte{byte(opcode.PUSH1)}
		w := &transaction.Witness{
			InvocationScript:   []byte{byte(opcode.NOP)},
			VerificationScript: verif,
		}
		_, err := bc.verifyHashAgainstScript(hash.Hash160(verif), w, ic, 1)
		require.True(t, errors.Is(err, ErrVerificationFailed))
	})
	t.Run("NoResult", func(t *testing.T) {
		verif := []byte{byte(opcode.DROP)}
		w := &transaction.Witness{
			InvocationScript:   []byte{byte(opcode.PUSH1)},
			VerificationScript: verif,
		}
		_, err := bc.verifyHashAgainstScript(hash.Hash160(verif), w, ic, gas)
		require.True(t, errors.Is(err, ErrVerificationFailed))
	})
	t.Run("BadResult", func(t *testing.T) {
		verif := make([]byte, 66)
		verif[0] = byte(opcode.PUSHDATA1)
		verif[1] = 64
		w := &transaction.Witness{
			InvocationScript:   []byte{byte(opcode.NOP)},
			VerificationScript: verif,
		}
		_, err := bc.verifyHashAgainstScript(hash.Hash160(verif), w, ic, gas)
		require.True(t, errors.Is(err, ErrVerificationFailed))
	})
	t.Run("TooManyResults", func(t *testing.T) {
		verif := []byte{byte(opcode.NOP)}
		w := &transaction.Witness{
			InvocationScript:   []byte{byte(opcode.PUSH1), byte(opcode.PUSH1)},
			VerificationScript: verif,
		}
		_, err := bc.verifyHashAgainstScript(hash.Hash160(verif), w, ic, gas)
		require.True(t, errors.Is(err, ErrVerificationFailed))
	})
}

func TestIsTxStillRelevant(t *testing.T) {
	bc := newTestChain(t)

	mp := bc.GetMemPool()
	newTx := func(t *testing.T) *transaction.Transaction {
		tx := transaction.New([]byte{byte(opcode.RET)}, 100)
		tx.ValidUntilBlock = bc.BlockHeight() + 1
		tx.Signers = []transaction.Signer{{
			Account: neoOwner,
			Scopes:  transaction.CalledByEntry,
		}}
		return tx
	}

	t.Run("small ValidUntilBlock", func(t *testing.T) {
		tx := newTx(t)
		require.NoError(t, testchain.SignTx(bc, tx))

		require.True(t, bc.IsTxStillRelevant(tx, nil, false))
		require.NoError(t, bc.AddBlock(bc.newBlock()))
		require.False(t, bc.IsTxStillRelevant(tx, nil, false))
	})

	t.Run("tx is already persisted", func(t *testing.T) {
		tx := newTx(t)
		tx.ValidUntilBlock = bc.BlockHeight() + 2
		require.NoError(t, testchain.SignTx(bc, tx))

		require.True(t, bc.IsTxStillRelevant(tx, nil, false))
		require.NoError(t, bc.AddBlock(bc.newBlock(tx)))
		require.False(t, bc.IsTxStillRelevant(tx, nil, false))
	})

	t.Run("tx with Conflicts attribute", func(t *testing.T) {
		tx1 := newTx(t)
		require.NoError(t, testchain.SignTx(bc, tx1))

		tx2 := newTx(t)
		tx2.Attributes = []transaction.Attribute{{
			Type:  transaction.ConflictsT,
			Value: &transaction.Conflicts{Hash: tx1.Hash()},
		}}
		require.NoError(t, testchain.SignTx(bc, tx2))

		require.True(t, bc.IsTxStillRelevant(tx1, mp, false))
		require.NoError(t, bc.verifyAndPoolTx(tx2, mp, bc))
		require.False(t, bc.IsTxStillRelevant(tx1, mp, false))
	})
	t.Run("NotValidBefore", func(t *testing.T) {
		tx3 := newTx(t)
		tx3.Attributes = []transaction.Attribute{{
			Type:  transaction.NotValidBeforeT,
			Value: &transaction.NotValidBefore{Height: bc.BlockHeight() + 1},
		}}
		tx3.ValidUntilBlock = bc.BlockHeight() + 2
		require.NoError(t, testchain.SignTx(bc, tx3))

		require.False(t, bc.IsTxStillRelevant(tx3, nil, false))
		require.NoError(t, bc.AddBlock(bc.newBlock()))
		require.True(t, bc.IsTxStillRelevant(tx3, nil, false))
	})
	t.Run("contract witness check fails", func(t *testing.T) {
		src := fmt.Sprintf(`package verify
		import (
			"github.com/nspcc-dev/neo-go/pkg/interop/contract"
			"github.com/nspcc-dev/neo-go/pkg/interop/util"
		)
		func Verify() bool {
			addr := util.FromAddress("`+address.Uint160ToString(bc.contracts.Ledger.Hash)+`")
			currentHeight := contract.Call(addr, "currentIndex", contract.ReadStates)
			return currentHeight.(int) < %d
		}`, bc.BlockHeight()+2) // deploy + next block
		txDeploy, h, _, err := testchain.NewDeployTx(bc, "TestVerify", neoOwner, strings.NewReader(src), nil)
		require.NoError(t, err)
		txDeploy.ValidUntilBlock = bc.BlockHeight() + 1
		addSigners(neoOwner, txDeploy)
		require.NoError(t, testchain.SignTx(bc, txDeploy))
		require.NoError(t, bc.AddBlock(bc.newBlock(txDeploy)))

		tx := newTx(t)
		tx.Signers = append(tx.Signers, transaction.Signer{
			Account: h,
			Scopes:  transaction.None,
		})
		tx.NetworkFee += 10_000_000
		require.NoError(t, testchain.SignTx(bc, tx))
		tx.Scripts = append(tx.Scripts, transaction.Witness{})

		require.True(t, bc.IsTxStillRelevant(tx, mp, false))
		require.NoError(t, bc.AddBlock(bc.newBlock()))
		require.False(t, bc.IsTxStillRelevant(tx, mp, false))
	})
}

func TestMemPoolRemoval(t *testing.T) {
	const added = 16
	const notAdded = 32
	bc := newTestChain(t)
	addedTxes := make([]*transaction.Transaction, added)
	notAddedTxes := make([]*transaction.Transaction, notAdded)
	for i := range addedTxes {
		addedTxes[i] = bc.newTestTx(testchain.MultisigScriptHash(), []byte{byte(opcode.PUSH1)})
		require.NoError(t, testchain.SignTx(bc, addedTxes[i]))
		require.NoError(t, bc.PoolTx(addedTxes[i]))
	}
	for i := range notAddedTxes {
		notAddedTxes[i] = bc.newTestTx(testchain.MultisigScriptHash(), []byte{byte(opcode.PUSH1)})
		require.NoError(t, testchain.SignTx(bc, notAddedTxes[i]))
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
	tx1 := transaction.New([]byte{byte(opcode.PUSH1)}, 0)
	tx1.ValidUntilBlock = 16
	tx1.Signers = []transaction.Signer{{
		Account: testchain.MultisigScriptHash(),
		Scopes:  transaction.CalledByEntry,
	}}
	tx2 := transaction.New([]byte{byte(opcode.PUSH2)}, 0)
	tx2.ValidUntilBlock = 16
	tx2.Signers = []transaction.Signer{{
		Account: testchain.MultisigScriptHash(),
		Scopes:  transaction.CalledByEntry,
	}}
	require.NoError(t, testchain.SignTx(bc, tx1, tx2))
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
		assert.Equal(t, txSize, tx.Size())
		assert.Equal(t, block.Transactions[0], tx)
		assert.Equal(t, 1, io.GetVarSize(tx.Attributes))
		assert.Equal(t, 1, io.GetVarSize(tx.Scripts))
		assert.NoError(t, bc.persist())
	}
}

func TestGetClaimable(t *testing.T) {
	bc := newTestChain(t)

	_, err := bc.genBlocks(10)
	require.NoError(t, err)

	t.Run("first generation period", func(t *testing.T) {
		amount, err := bc.CalculateClaimable(neoOwner, 1)
		require.NoError(t, err)
		require.EqualValues(t, big.NewInt(5*native.GASFactor/10), amount)
	})
}

func TestClose(t *testing.T) {
	defer func() {
		r := recover()
		assert.NotNil(t, r)
	}()
	bc := initTestChain(t, nil, nil)
	go bc.Run()
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
	assert.Len(t, notificationCh, 1) // validator bounty
	assert.Len(t, executionCh, 2)
	assert.Empty(t, txCh)

	b := <-blockCh
	assert.Equal(t, blocks[0], b)
	assert.Empty(t, blockCh)

	aer := <-executionCh
	assert.Equal(t, b.Hash(), aer.Container)
	aer = <-executionCh
	assert.Equal(t, b.Hash(), aer.Container)

	notif := <-notificationCh
	require.Equal(t, bc.UtilityTokenHash(), notif.ScriptHash)

	script := io.NewBufBinWriter()
	emit.Bytes(script.BinWriter, []byte("yay!"))
	emit.Syscall(script.BinWriter, interopnames.SystemRuntimeNotify)
	require.NoError(t, script.Err)
	txGood1 := transaction.New(script.Bytes(), 0)
	txGood1.Signers = []transaction.Signer{{Account: neoOwner}}
	txGood1.Nonce = 1
	txGood1.ValidUntilBlock = 1024
	require.NoError(t, testchain.SignTx(bc, txGood1))

	// Reset() reuses the script buffer and we need to keep scripts.
	script = io.NewBufBinWriter()
	emit.Bytes(script.BinWriter, []byte("nay!"))
	emit.Syscall(script.BinWriter, interopnames.SystemRuntimeNotify)
	emit.Opcodes(script.BinWriter, opcode.THROW)
	require.NoError(t, script.Err)
	txBad := transaction.New(script.Bytes(), 0)
	txBad.Signers = []transaction.Signer{{Account: neoOwner}}
	txBad.Nonce = 2
	txBad.ValidUntilBlock = 1024
	require.NoError(t, testchain.SignTx(bc, txBad))

	script = io.NewBufBinWriter()
	emit.Bytes(script.BinWriter, []byte("yay! yay! yay!"))
	emit.Syscall(script.BinWriter, interopnames.SystemRuntimeNotify)
	require.NoError(t, script.Err)
	txGood2 := transaction.New(script.Bytes(), 0)
	txGood2.Signers = []transaction.Signer{{Account: neoOwner}}
	txGood2.Nonce = 3
	txGood2.ValidUntilBlock = 1024
	require.NoError(t, testchain.SignTx(bc, txGood2))

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
	require.Equal(t, b.Hash(), exec.Container)
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
		require.Equal(t, tx.Hash(), exec.Container)
		if exec.VMState == vm.HaltState {
			notif := <-notificationCh
			require.Equal(t, hash.Hash160(tx.Script), notif.ScriptHash)
		}
	}
	assert.Empty(t, txCh)
	assert.Len(t, notificationCh, 1)
	assert.Len(t, executionCh, 1)

	notif = <-notificationCh
	require.Equal(t, bc.UtilityTokenHash(), notif.ScriptHash)

	exec = <-executionCh
	require.Equal(t, b.Hash(), exec.Container)
	require.Equal(t, exec.VMState, vm.HaltState)

	bc.UnsubscribeFromBlocks(blockCh)
	bc.UnsubscribeFromTransactions(txCh)
	bc.UnsubscribeFromNotifications(notificationCh)
	bc.UnsubscribeFromExecutions(executionCh)

	// Ensure that new blocks are processed correctly after unsubscription.
	_, err = bc.genBlocks(2 * chBufSize)
	require.NoError(t, err)
}

func testDumpAndRestore(t *testing.T, dumpF, restoreF func(c *config.Config)) {
	if restoreF == nil {
		restoreF = dumpF
	}

	bc := newTestChainWithCustomCfg(t, dumpF)

	initBasicChain(t, bc)
	require.True(t, bc.BlockHeight() > 5) // ensure that test is valid

	w := io.NewBufBinWriter()
	require.NoError(t, chaindump.Dump(bc, w.BinWriter, 0, bc.BlockHeight()+1))
	require.NoError(t, w.Err)

	buf := w.Bytes()
	t.Run("invalid start", func(t *testing.T) {
		bc2 := newTestChainWithCustomCfg(t, restoreF)

		r := io.NewBinReaderFromBuf(buf)
		require.Error(t, chaindump.Restore(bc2, r, 2, 1, nil))
	})
	t.Run("good", func(t *testing.T) {
		bc2 := newTestChainWithCustomCfg(t, restoreF)

		r := io.NewBinReaderFromBuf(buf)
		require.NoError(t, chaindump.Restore(bc2, r, 0, 2, nil))
		require.Equal(t, uint32(1), bc2.BlockHeight())

		r = io.NewBinReaderFromBuf(buf) // new reader because start is relative to dump
		require.NoError(t, chaindump.Restore(bc2, r, 2, 1, nil))
		t.Run("check handler", func(t *testing.T) {
			lastIndex := uint32(0)
			errStopped := errors.New("stopped")
			f := func(b *block.Block) error {
				lastIndex = b.Index
				if b.Index >= bc.BlockHeight()-1 {
					return errStopped
				}
				return nil
			}
			require.NoError(t, chaindump.Restore(bc2, r, 0, 1, f))
			require.Equal(t, bc2.BlockHeight(), lastIndex)

			r = io.NewBinReaderFromBuf(buf)
			err := chaindump.Restore(bc2, r, 4, bc.BlockHeight()-bc2.BlockHeight(), f)
			require.True(t, errors.Is(err, errStopped))
			require.Equal(t, bc.BlockHeight()-1, lastIndex)
		})
	})

}

func TestDumpAndRestore(t *testing.T) {
	t.Run("no state root", func(t *testing.T) {
		testDumpAndRestore(t, func(c *config.Config) {
			c.ProtocolConfiguration.StateRootInHeader = false
		}, nil)
	})
	t.Run("with state root", func(t *testing.T) {
		testDumpAndRestore(t, func(c *config.Config) {
			c.ProtocolConfiguration.StateRootInHeader = false
		}, nil)
	})
	t.Run("remove untraceable", func(t *testing.T) {
		// Dump can only be created if all blocks and transactions are present.
		testDumpAndRestore(t, nil, func(c *config.Config) {
			c.ProtocolConfiguration.MaxTraceableBlocks = 2
			c.ProtocolConfiguration.RemoveUntraceableBlocks = true
		})
	})
}

func TestRemoveUntraceable(t *testing.T) {
	bc := newTestChainWithCustomCfg(t, func(c *config.Config) {
		c.ProtocolConfiguration.MaxTraceableBlocks = 2
		c.ProtocolConfiguration.RemoveUntraceableBlocks = true
	})

	tx1, err := testchain.NewTransferFromOwner(bc, bc.contracts.NEO.Hash, util.Uint160{}, 1, 0, bc.BlockHeight()+1)
	require.NoError(t, err)
	b1 := bc.newBlock(tx1)
	require.NoError(t, bc.AddBlock(b1))
	tx1Height := bc.BlockHeight()

	tx2, err := testchain.NewTransferFromOwner(bc, bc.contracts.NEO.Hash, util.Uint160{}, 1, 0, bc.BlockHeight()+1)
	require.NoError(t, err)
	require.NoError(t, bc.AddBlock(bc.newBlock(tx2)))

	_, h1, err := bc.GetTransaction(tx1.Hash())
	require.NoError(t, err)
	require.Equal(t, tx1Height, h1)

	require.NoError(t, bc.AddBlock(bc.newBlock()))

	_, _, err = bc.GetTransaction(tx1.Hash())
	require.Error(t, err)
	_, err = bc.GetAppExecResults(tx1.Hash(), trigger.Application)
	require.Error(t, err)
	_, err = bc.GetBlock(b1.Hash())
	require.Error(t, err)
	_, err = bc.GetHeader(b1.Hash())
	require.NoError(t, err)
}

func TestInvalidNotification(t *testing.T) {
	bc := newTestChain(t)

	cs, _ := getTestContractState(bc)
	require.NoError(t, bc.contracts.Management.PutContractState(bc.dao, cs))

	aer, err := invokeContractMethod(bc, 1_00000000, cs.Hash, "invalidStack")
	require.NoError(t, err)
	require.Equal(t, 2, len(aer.Stack))
	require.Nil(t, aer.Stack[0])
	require.Equal(t, stackitem.InteropT, aer.Stack[1].Type())
}

// Test that deletion of non-existent doesn't result in error in tx or block addition.
func TestMPTDeleteNoKey(t *testing.T) {
	bc := newTestChain(t)

	cs, _ := getTestContractState(bc)
	require.NoError(t, bc.contracts.Management.PutContractState(bc.dao, cs))
	aer, err := invokeContractMethod(bc, 1_00000000, cs.Hash, "delValue", "non-existent-key")
	require.NoError(t, err)
	require.Equal(t, vm.HaltState, aer.VMState)
}

// Test that UpdateHistory is added to ProtocolConfiguration for all native contracts
// for all default configurations. If UpdateHistory is not added to config, then
// native contract is disabled. It's easy to forget about config while adding new
// native contract.
func TestConfigNativeUpdateHistory(t *testing.T) {
	const prefixPath = "../../config"
	check := func(t *testing.T, cfgFileSuffix interface{}) {
		cfgPath := path.Join(prefixPath, fmt.Sprintf("protocol.%s.yml", cfgFileSuffix))
		cfg, err := config.LoadFile(cfgPath)
		require.NoError(t, err, fmt.Errorf("failed to load %s", cfgPath))
		natives := native.NewContracts(cfg.ProtocolConfiguration.P2PSigExtensions, map[string][]uint32{})
		assert.Equal(t, len(natives.Contracts),
			len(cfg.ProtocolConfiguration.NativeUpdateHistories),
			fmt.Errorf("protocol configuration file %s: extra or missing NativeUpdateHistory in NativeActivations section", cfgPath))
		for _, c := range natives.Contracts {
			assert.NotNil(t, cfg.ProtocolConfiguration.NativeUpdateHistories[c.Metadata().Name],
				fmt.Errorf("protocol configuration file %s: configuration for %s native contract is missing in NativeActivations section; "+
					"edit the test if the contract should be disabled", cfgPath, c.Metadata().Name))
		}
	}
	testCases := []interface{}{
		netmode.MainNet,
		netmode.PrivNet,
		netmode.TestNet,
		netmode.UnitTestNet,
		"privnet.docker.one",
		"privnet.docker.two",
		"privnet.docker.three",
		"privnet.docker.four",
		"privnet.docker.single",
		"unit_testnet.single",
	}
	for _, tc := range testCases {
		check(t, tc)
	}
}
