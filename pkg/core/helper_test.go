package core

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

var privNetKeys = []string{
	"KxyjQ8eUa4FHt3Gvioyt1Wz29cTUrE4eTqX3yFSk1YFCsPL8uNsY",
	"KzfPUYDC9n2yf4fK5ro4C8KMcdeXtFuEnStycbZgX3GomiUsvX6W",
	"KzgWE3u3EDp13XPXXuTKZxeJ3Gi8Bsm8f9ijY3ZsCKKRvZUo1Cdn",
	"L2oEXKRAAMiPEZukwR5ho2S6SMeQLhcK9mF71ZnF7GvT8dU4Kkgz",
}

// newTestChain should be called before newBlock invocation to properly setup
// global state.
func newTestChain(t *testing.T) *Blockchain {
	unitTestNetCfg, err := config.Load("../../config", config.ModeUnitTestNet)
	require.NoError(t, err)
	chain, err := NewBlockchain(storage.NewMemoryStore(), unitTestNetCfg.ProtocolConfiguration, zaptest.NewLogger(t))
	require.NoError(t, err)
	go chain.Run()
	return chain
}

func (bc *Blockchain) newBlock(txs ...*transaction.Transaction) *block.Block {
	lastBlock := bc.topBlock.Load().(*block.Block)
	return newBlock(bc.config, lastBlock.Index+1, lastBlock.Hash(), txs...)
}

func newBlock(cfg config.ProtocolConfiguration, index uint32, prev util.Uint256, txs ...*transaction.Transaction) *block.Block {
	validators, _ := getValidators(cfg)
	vlen := len(validators)
	valScript, _ := smartcontract.CreateMultiSigRedeemScript(
		vlen-(vlen-1)/3,
		validators,
	)
	witness := transaction.Witness{
		VerificationScript: valScript,
	}
	b := &block.Block{
		Base: block.Base{
			Version:       0,
			PrevHash:      prev,
			Timestamp:     uint32(time.Now().UTC().Unix()) + index,
			Index:         index,
			ConsensusData: 1111,
			NextConsensus: witness.ScriptHash(),
			Script:        witness,
		},
		Transactions: txs,
	}
	_ = b.RebuildMerkleRoot()

	invScript := make([]byte, 0)
	for _, wif := range privNetKeys {
		pKey, err := keys.NewPrivateKeyFromWIF(wif)
		if err != nil {
			panic(err)
		}
		b := b.GetSignedPart()
		sig := pKey.Sign(b)
		if len(sig) != 64 {
			panic("wrong signature length")
		}
		invScript = append(invScript, byte(opcode.PUSHBYTES64))
		invScript = append(invScript, sig...)
	}
	b.Script.InvocationScript = invScript
	return b
}

func (bc *Blockchain) genBlocks(n int) ([]*block.Block, error) {
	blocks := make([]*block.Block, n)
	lastHash := bc.topBlock.Load().(*block.Block).Hash()
	lastIndex := bc.topBlock.Load().(*block.Block).Index
	for i := 0; i < n; i++ {
		minerTx := transaction.NewMinerTXWithNonce(uint32(1234 + i))
		minerTx.ValidUntilBlock = lastIndex + uint32(i) + 1
		blocks[i] = newBlock(bc.config, uint32(i)+lastIndex+1, lastHash, minerTx)
		if err := bc.AddBlock(blocks[i]); err != nil {
			return blocks, err
		}
		lastHash = blocks[i].Hash()
	}
	return blocks, nil
}

func getDecodedBlock(t *testing.T, i int) *block.Block {
	data, err := getBlockData(i)
	require.NoError(t, err)

	b, err := hex.DecodeString(data["raw"].(string))
	require.NoError(t, err)

	block := &block.Block{}
	require.NoError(t, testserdes.DecodeBinary(b, block))

	return block
}

func getBlockData(i int) (map[string]interface{}, error) {
	b, err := ioutil.ReadFile(fmt.Sprintf("test_data/block_%d.json", i))
	if err != nil {
		return nil, err
	}
	var data map[string]interface{}
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, err
	}
	return data, err
}

func newDumbBlock() *block.Block {
	return &block.Block{
		Base: block.Base{
			Version:       0,
			PrevHash:      hash.Sha256([]byte("a")),
			MerkleRoot:    hash.Sha256([]byte("b")),
			Timestamp:     uint32(100500),
			Index:         1,
			ConsensusData: 1111,
			NextConsensus: hash.Hash160([]byte("a")),
			Script: transaction.Witness{
				VerificationScript: []byte{0x51}, // PUSH1
				InvocationScript:   []byte{0x61}, // NOP
			},
		},
		Transactions: []*transaction.Transaction{
			{Type: transaction.MinerType},
			{Type: transaction.IssueType},
		},
	}
}

func getInvocationScript(data []byte, priv *keys.PrivateKey) []byte {
	signature := priv.Sign(data)
	return append([]byte{byte(opcode.PUSHBYTES64)}, signature...)
}

// This function generates "../rpc/testdata/testblocks.acc" file which contains data
// for RPC unit tests. It also is a nice integration test.
// To generate new "../rpc/testdata/testblocks.acc", follow the steps:
// 		1. Set saveChain down below to true
// 		2. Run tests with `$ make test`
func TestCreateBasicChain(t *testing.T) {
	const saveChain = false
	const prefix = "../rpc/server/testdata/"
	// To make enough GAS.
	const numOfEmptyBlocks = 200
	// Increase in case if you need more blocks
	const validUntilBlock = numOfEmptyBlocks + 1000

	// To be incremented after each created transaction to keep chain constant.
	var testNonce uint32 = 1

	// Use as nonce when new transaction is created to avoid random data in tests.
	getNextNonce := func() uint32 {
		testNonce++
		return testNonce
	}

	// Creates new miner tx with specified validUntilBlock field
	nextMinerTx := func(validUntilBlock uint32) *transaction.Transaction {
		minerTx := transaction.NewMinerTXWithNonce(getNextNonce())
		minerTx.ValidUntilBlock = validUntilBlock
		return minerTx
	}

	var neoAmount = util.Fixed8FromInt64(99999000)
	var neoRemainder = util.Fixed8FromInt64(100000000) - neoAmount
	bc := newTestChain(t)

	// Move almost all NEO to one simple account.
	txMoveNeo := transaction.NewContractTX()
	txMoveNeo.ValidUntilBlock = validUntilBlock
	txMoveNeo.Nonce = getNextNonce()

	// use output of issue tx from genesis block as an input
	genesisBlock, err := bc.GetBlock(bc.GetHeaderHash(0))
	require.NoError(t, err)
	require.Equal(t, 4, len(genesisBlock.Transactions))
	h := genesisBlock.Transactions[3].Hash()
	txMoveNeo.AddInput(&transaction.Input{
		PrevHash:  h,
		PrevIndex: 0,
	})

	// multisig address which possess all NEO
	scriptHash, err := util.Uint160DecodeStringBE("d60ac443bb800fb08261e75fa5925d747d485861")
	require.NoError(t, err)
	priv0, err := keys.NewPrivateKeyFromWIF(privNetKeys[0])
	require.NoError(t, err)
	txMoveNeo.AddOutput(&transaction.Output{
		AssetID:    GoverningTokenID(),
		Amount:     neoAmount,
		ScriptHash: priv0.GetScriptHash(),
		Position:   0,
	})
	txMoveNeo.AddOutput(&transaction.Output{
		AssetID:    GoverningTokenID(),
		Amount:     neoRemainder,
		ScriptHash: scriptHash,
		Position:   1,
	})
	txMoveNeo.Data = new(transaction.ContractTX)

	validators, err := getValidators(bc.config)
	require.NoError(t, err)
	rawScript, err := smartcontract.CreateMultiSigRedeemScript(len(bc.config.StandbyValidators)/2+1, validators)
	require.NoError(t, err)
	data := txMoveNeo.GetSignedPart()

	var invoc []byte
	for i := range privNetKeys {
		priv, err := keys.NewPrivateKeyFromWIF(privNetKeys[i])
		require.NoError(t, err)
		invoc = append(invoc, getInvocationScript(data, priv)...)
	}

	txMoveNeo.Scripts = []transaction.Witness{{
		InvocationScript:   invoc,
		VerificationScript: rawScript,
	}}
	b := bc.newBlock(nextMinerTx(validUntilBlock), txMoveNeo)
	require.NoError(t, bc.AddBlock(b))
	t.Logf("txMoveNeo: %s", txMoveNeo.Hash().StringLE())

	// info for getblockheader rpc tests
	t.Logf("header hash: %s", b.Hash().StringLE())
	buf := io.NewBufBinWriter()
	b.Header().EncodeBinary(buf.BinWriter)
	t.Logf("header: %s", hex.EncodeToString(buf.Bytes()))

	// Generate some blocks to be able to claim GAS for them.
	_, err = bc.genBlocks(numOfEmptyBlocks)
	require.NoError(t, err)

	acc0, err := wallet.NewAccountFromWIF(priv0.WIF())
	require.NoError(t, err)

	// Make a NEO roundtrip (send to myself) and claim GAS.
	txNeoRound := transaction.NewContractTX()
	txNeoRound.Nonce = getNextNonce()
	txNeoRound.ValidUntilBlock = validUntilBlock
	txNeoRound.AddInput(&transaction.Input{
		PrevHash:  txMoveNeo.Hash(),
		PrevIndex: 0,
	})
	txNeoRound.AddOutput(&transaction.Output{
		AssetID:    GoverningTokenID(),
		Amount:     neoAmount,
		ScriptHash: priv0.GetScriptHash(),
		Position:   0,
	})
	txNeoRound.Data = new(transaction.ContractTX)
	require.NoError(t, acc0.SignTx(txNeoRound))
	b = bc.newBlock(nextMinerTx(validUntilBlock), txNeoRound)
	require.NoError(t, bc.AddBlock(b))
	t.Logf("txNeoRound: %s", txNeoRound.Hash().StringLE())

	claim := new(transaction.ClaimTX)
	claim.Claims = append(claim.Claims, transaction.Input{
		PrevHash:  txMoveNeo.Hash(),
		PrevIndex: 0,
	})
	txClaim := transaction.NewClaimTX(claim)
	txClaim.Nonce = getNextNonce()
	txClaim.ValidUntilBlock = validUntilBlock
	txClaim.Data = claim
	neoGas, sysGas, err := bc.CalculateClaimable(neoAmount, 1, bc.BlockHeight())
	require.NoError(t, err)
	gasOwned := neoGas + sysGas
	txClaim.AddOutput(&transaction.Output{
		AssetID:    UtilityTokenID(),
		Amount:     gasOwned,
		ScriptHash: priv0.GetScriptHash(),
		Position:   0,
	})
	require.NoError(t, acc0.SignTx(txClaim))
	b = bc.newBlock(nextMinerTx(validUntilBlock), txClaim)
	require.NoError(t, bc.AddBlock(b))
	t.Logf("txClaim: %s", txClaim.Hash().StringLE())

	// Push some contract into the chain.
	avm, err := ioutil.ReadFile(prefix + "test_contract.avm")
	require.NoError(t, err)
	t.Logf("contractHash: %s", hash.Hash160(avm).StringLE())

	var props smartcontract.PropertyState
	script := io.NewBufBinWriter()
	emit.Bytes(script.BinWriter, []byte("Da contract dat hallos u"))
	emit.Bytes(script.BinWriter, []byte("joe@example.com"))
	emit.Bytes(script.BinWriter, []byte("Random Guy"))
	emit.Bytes(script.BinWriter, []byte("0.99"))
	emit.Bytes(script.BinWriter, []byte("Helloer"))
	props |= smartcontract.HasStorage
	emit.Int(script.BinWriter, int64(props))
	emit.Int(script.BinWriter, int64(5))
	params := make([]byte, 1)
	params[0] = byte(7)
	emit.Bytes(script.BinWriter, params)
	emit.Bytes(script.BinWriter, avm)
	emit.Syscall(script.BinWriter, "Neo.Contract.Create")
	txScript := script.Bytes()

	invFee := util.Fixed8FromFloat(100)
	txDeploy := transaction.NewInvocationTX(txScript, invFee)
	txDeploy.Nonce = getNextNonce()
	txDeploy.ValidUntilBlock = validUntilBlock
	txDeploy.AddInput(&transaction.Input{
		PrevHash:  txClaim.Hash(),
		PrevIndex: 0,
	})
	txDeploy.AddOutput(&transaction.Output{
		AssetID:    UtilityTokenID(),
		Amount:     gasOwned - invFee,
		ScriptHash: priv0.GetScriptHash(),
		Position:   0,
	})
	gasOwned -= invFee
	require.NoError(t, acc0.SignTx(txDeploy))
	b = bc.newBlock(nextMinerTx(validUntilBlock), txDeploy)
	require.NoError(t, bc.AddBlock(b))
	t.Logf("txDeploy: %s", txDeploy.Hash().StringLE())

	// Now invoke this contract.
	script = io.NewBufBinWriter()
	emit.AppCallWithOperationAndArgs(script.BinWriter, hash.Hash160(avm), "Put", "testkey", "testvalue")

	txInv := transaction.NewInvocationTX(script.Bytes(), 0)
	txInv.Nonce = getNextNonce()
	txInv.ValidUntilBlock = validUntilBlock
	b = bc.newBlock(nextMinerTx(validUntilBlock), txInv)
	require.NoError(t, bc.AddBlock(b))
	t.Logf("txInv: %s", txInv.Hash().StringLE())

	priv1, err := keys.NewPrivateKeyFromWIF(privNetKeys[1])
	require.NoError(t, err)
	txNeo0to1 := transaction.NewContractTX()
	txNeo0to1.Nonce = getNextNonce()
	txNeo0to1.ValidUntilBlock = validUntilBlock
	txNeo0to1.Data = new(transaction.ContractTX)
	txNeo0to1.AddInput(&transaction.Input{
		PrevHash:  txNeoRound.Hash(),
		PrevIndex: 0,
	})
	txNeo0to1.AddOutput(&transaction.Output{
		AssetID:    GoverningTokenID(),
		Amount:     util.Fixed8FromInt64(1000),
		ScriptHash: priv1.GetScriptHash(),
	})
	txNeo0to1.AddOutput(&transaction.Output{
		AssetID:    GoverningTokenID(),
		Amount:     neoAmount - util.Fixed8FromInt64(1000),
		ScriptHash: priv0.GetScriptHash(),
	})

	require.NoError(t, acc0.SignTx(txNeo0to1))
	b = bc.newBlock(nextMinerTx(validUntilBlock), txNeo0to1)
	require.NoError(t, bc.AddBlock(b))

	sh := hash.Hash160(avm)
	w := io.NewBufBinWriter()
	emit.AppCallWithOperationAndArgs(w.BinWriter, sh, "init")
	initTx := transaction.NewInvocationTX(w.Bytes(), 0)
	initTx.Nonce = getNextNonce()
	initTx.ValidUntilBlock = validUntilBlock
	transferTx := newNEP5Transfer(sh, sh, priv0.GetScriptHash(), 1000)
	transferTx.Nonce = getNextNonce()
	transferTx.ValidUntilBlock = validUntilBlock

	b = bc.newBlock(nextMinerTx(validUntilBlock), initTx, transferTx)
	require.NoError(t, bc.AddBlock(b))

	transferTx = newNEP5Transfer(sh, priv0.GetScriptHash(), priv1.GetScriptHash(), 123)
	transferTx.Nonce = getNextNonce()
	transferTx.ValidUntilBlock = validUntilBlock
	b = bc.newBlock(nextMinerTx(validUntilBlock), transferTx)
	require.NoError(t, bc.AddBlock(b))

	if saveChain {
		outStream, err := os.Create(prefix + "testblocks.acc")
		require.NoError(t, err)
		defer outStream.Close()

		writer := io.NewBinWriterFromIO(outStream)

		count := bc.BlockHeight() + 1
		writer.WriteU32LE(count - 1)

		for i := 1; i < int(count); i++ {
			bh := bc.GetHeaderHash(i)
			b, err := bc.GetBlock(bh)
			require.NoError(t, err)
			bytes, err := testserdes.EncodeBinary(b)
			require.NoError(t, err)
			writer.WriteU32LE(uint32(len(bytes)))
			writer.WriteBytes(bytes)
			require.NoError(t, writer.Err)
		}
	}

	// Make a NEO roundtrip (send to myself) and claim GAS.
	txNeoRound = transaction.NewContractTX()
	txNeoRound.Nonce = getNextNonce()
	txNeoRound.ValidUntilBlock = validUntilBlock
	txNeoRound.AddInput(&transaction.Input{
		PrevHash:  txNeo0to1.Hash(),
		PrevIndex: 1,
	})
	txNeoRound.AddOutput(&transaction.Output{
		AssetID:    GoverningTokenID(),
		Amount:     neoAmount - util.Fixed8FromInt64(1000),
		ScriptHash: priv0.GetScriptHash(),
		Position:   0,
	})
	txNeoRound.Data = new(transaction.ContractTX)
	require.NoError(t, acc0.SignTx(txNeoRound))
	bw := io.NewBufBinWriter()
	txNeoRound.EncodeBinary(bw.BinWriter)
	t.Logf("sendrawtransaction: %s", hex.EncodeToString(bw.Bytes()))

	// Blocks for `submitblock` test. If you are planning to modify test chain from `testblocks.acc`,
	// please, update params value of `empty block` and `positive` tests.
	var blocks []*block.Block
	blocks = append(blocks, bc.newBlock(), bc.newBlock(nextMinerTx(validUntilBlock)))
	for i, b := range blocks {
		data, err := testserdes.EncodeBinary(b)
		require.NoError(t, err)
		t.Logf("\nblock %v for submitblock test:\n%s", i, hex.EncodeToString(data))
	}
}

func newNEP5Transfer(sc, from, to util.Uint160, amount int64) *transaction.Transaction {
	w := io.NewBufBinWriter()
	emit.AppCallWithOperationAndArgs(w.BinWriter, sc, "transfer", from, to, amount)
	emit.Opcode(w.BinWriter, opcode.THROWIFNOT)

	script := w.Bytes()
	return transaction.NewInvocationTX(script, 0)
}
