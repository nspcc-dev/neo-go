package core

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/interopnames"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/internal/testchain"
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

// multisig address which possess all NEO
var neoOwner = testchain.MultisigScriptHash()

// newTestChain should be called before newBlock invocation to properly setup
// global state.
func newTestChain(t *testing.T) *Blockchain {
	unitTestNetCfg, err := config.Load("../../config", testchain.Network())
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
	validators, _ := validatorsFromConfig(cfg)
	valScript, _ := smartcontract.CreateDefaultMultiSigRedeemScript(validators)
	witness := transaction.Witness{
		VerificationScript: valScript,
	}
	b := &block.Block{
		Base: block.Base{
			Network:       testchain.Network(),
			Version:       0,
			PrevHash:      prev,
			Timestamp:     uint64(time.Now().UTC().Unix())*1000 + uint64(index),
			Index:         index,
			NextConsensus: witness.ScriptHash(),
			Script:        witness,
		},
		ConsensusData: block.ConsensusData{
			PrimaryIndex: 0,
			Nonce:        1111,
		},
		Transactions: txs,
	}
	err := b.RebuildMerkleRoot()
	if err != nil {
		panic(err)
	}

	b.Script.InvocationScript = testchain.Sign(b.GetSignedPart())
	return b
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

func getDecodedBlock(t *testing.T, i int) *block.Block {
	data, err := getBlockData(i)
	require.NoError(t, err)

	b, err := hex.DecodeString(data["raw"].(string))
	require.NoError(t, err)

	block := block.New(testchain.Network())
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
			Network:       testchain.Network(),
			Version:       0,
			PrevHash:      hash.Sha256([]byte("a")),
			MerkleRoot:    hash.Sha256([]byte("b")),
			Timestamp:     100500,
			Index:         1,
			NextConsensus: hash.Hash160([]byte("a")),
			Script: transaction.Witness{
				VerificationScript: []byte{0x51}, // PUSH1
				InvocationScript:   []byte{0x61}, // NOP
			},
		},
		ConsensusData: block.ConsensusData{
			PrimaryIndex: 0,
			Nonce:        1111,
		},
		Transactions: []*transaction.Transaction{
			transaction.New(testchain.Network(), []byte{byte(opcode.PUSH1)}, 0),
		},
	}
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

	const neoAmount = 99999000
	bc := newTestChain(t)
	defer bc.Close()

	gasHash := bc.contracts.GAS.Hash
	neoHash := bc.contracts.NEO.Hash
	policyHash := bc.contracts.Policy.Hash
	t.Logf("native GAS hash: %v", gasHash)
	t.Logf("native NEO hash: %v", neoHash)
	t.Logf("native Policy hash: %v", policyHash)

	priv0 := testchain.PrivateKeyByID(0)
	priv0ScriptHash := priv0.GetScriptHash()

	require.Equal(t, big.NewInt(0), bc.GetUtilityTokenBalance(priv0ScriptHash))
	// Move some NEO to one simple account.
	txMoveNeo := newNEP5Transfer(neoHash, neoOwner, priv0ScriptHash, neoAmount)
	txMoveNeo.ValidUntilBlock = validUntilBlock
	txMoveNeo.Nonce = getNextNonce()
	txMoveNeo.Signers = []transaction.Signer{{
		Account:          neoOwner,
		Scopes:           transaction.CalledByEntry,
		AllowedContracts: nil,
		AllowedGroups:    nil,
	}}
	require.NoError(t, signTx(bc, txMoveNeo))
	// Move some GAS to one simple account.
	txMoveGas := newNEP5Transfer(gasHash, neoOwner, priv0ScriptHash, int64(util.Fixed8FromInt64(1000)))
	txMoveGas.ValidUntilBlock = validUntilBlock
	txMoveGas.Nonce = getNextNonce()
	txMoveGas.Signers = []transaction.Signer{{
		Account:          neoOwner,
		Scopes:           transaction.CalledByEntry,
		AllowedContracts: nil,
		AllowedGroups:    nil,
	}}
	require.NoError(t, signTx(bc, txMoveGas))
	b := bc.newBlock(txMoveNeo, txMoveGas)
	require.NoError(t, bc.AddBlock(b))
	t.Logf("Block1 hash: %s", b.Hash().StringLE())
	bw := io.NewBufBinWriter()
	b.EncodeBinary(bw.BinWriter)
	require.NoError(t, bw.Err)
	t.Logf("Block1 hex: %s", bw.Bytes())
	t.Logf("txMoveNeo hash: %s", txMoveNeo.Hash().StringLE())
	t.Logf("txMoveNeo hex: %s", hex.EncodeToString(txMoveNeo.Bytes()))
	t.Logf("txMoveGas hash: %s", txMoveGas.Hash().StringLE())

	require.True(t, bc.GetUtilityTokenBalance(priv0ScriptHash).Cmp(big.NewInt(1000*native.GASFactor)) >= 0)
	// info for getblockheader rpc tests
	t.Logf("header hash: %s", b.Hash().StringLE())
	buf := io.NewBufBinWriter()
	b.Header().EncodeBinary(buf.BinWriter)
	t.Logf("header: %s", hex.EncodeToString(buf.Bytes()))

	acc0, err := wallet.NewAccountFromWIF(priv0.WIF())
	require.NoError(t, err)

	// Push some contract into the chain.
	name := prefix + "test_contract.go"
	c, err := ioutil.ReadFile(name)
	require.NoError(t, err)
	avm, di, err := compiler.CompileWithDebugInfo(name, bytes.NewReader(c))
	require.NoError(t, err)
	t.Logf("contractHash: %s", hash.Hash160(avm).StringLE())

	script := io.NewBufBinWriter()
	m, err := di.ConvertToManifest(smartcontract.HasStorage, nil)
	require.NoError(t, err)
	bs, err := m.MarshalJSON()
	require.NoError(t, err)
	emit.Bytes(script.BinWriter, bs)
	emit.Bytes(script.BinWriter, avm)
	emit.Syscall(script.BinWriter, interopnames.SystemContractCreate)
	txScript := script.Bytes()

	txDeploy := transaction.New(testchain.Network(), txScript, 100*native.GASFactor)
	txDeploy.Nonce = getNextNonce()
	txDeploy.ValidUntilBlock = validUntilBlock
	txDeploy.Signers = []transaction.Signer{{Account: priv0ScriptHash}}
	require.NoError(t, addNetworkFee(bc, txDeploy, acc0))
	require.NoError(t, acc0.SignTx(txDeploy))
	b = bc.newBlock(txDeploy)
	require.NoError(t, bc.AddBlock(b))
	t.Logf("txDeploy: %s", txDeploy.Hash().StringLE())
	t.Logf("Block2 hash: %s", b.Hash().StringLE())

	// Now invoke this contract.
	script = io.NewBufBinWriter()
	emit.AppCallWithOperationAndArgs(script.BinWriter, hash.Hash160(avm), "putValue", "testkey", "testvalue")

	txInv := transaction.New(testchain.Network(), script.Bytes(), 1*native.GASFactor)
	txInv.Nonce = getNextNonce()
	txInv.ValidUntilBlock = validUntilBlock
	txInv.Signers = []transaction.Signer{{Account: priv0ScriptHash}}
	require.NoError(t, addNetworkFee(bc, txInv, acc0))
	require.NoError(t, acc0.SignTx(txInv))
	b = bc.newBlock(txInv)
	require.NoError(t, bc.AddBlock(b))
	t.Logf("txInv: %s", txInv.Hash().StringLE())

	priv1 := testchain.PrivateKeyByID(1)
	txNeo0to1 := newNEP5Transfer(neoHash, priv0ScriptHash, priv1.GetScriptHash(), 1000)
	txNeo0to1.Nonce = getNextNonce()
	txNeo0to1.ValidUntilBlock = validUntilBlock
	txNeo0to1.Signers = []transaction.Signer{
		{
			Account:          priv0ScriptHash,
			Scopes:           transaction.CalledByEntry,
			AllowedContracts: nil,
			AllowedGroups:    nil,
		},
	}
	require.NoError(t, addNetworkFee(bc, txNeo0to1, acc0))
	require.NoError(t, acc0.SignTx(txNeo0to1))
	b = bc.newBlock(txNeo0to1)
	require.NoError(t, bc.AddBlock(b))

	sh := hash.Hash160(avm)
	w := io.NewBufBinWriter()
	emit.AppCallWithOperationAndArgs(w.BinWriter, sh, "init")
	initTx := transaction.New(testchain.Network(), w.Bytes(), 1*native.GASFactor)
	initTx.Nonce = getNextNonce()
	initTx.ValidUntilBlock = validUntilBlock
	initTx.Signers = []transaction.Signer{{Account: priv0ScriptHash}}
	require.NoError(t, addNetworkFee(bc, initTx, acc0))
	require.NoError(t, acc0.SignTx(initTx))
	transferTx := newNEP5Transfer(sh, sh, priv0.GetScriptHash(), 1000)
	transferTx.Nonce = getNextNonce()
	transferTx.ValidUntilBlock = validUntilBlock
	transferTx.Signers = []transaction.Signer{
		{
			Account:          priv0ScriptHash,
			Scopes:           transaction.CalledByEntry,
			AllowedContracts: nil,
			AllowedGroups:    nil,
		},
	}
	require.NoError(t, addNetworkFee(bc, transferTx, acc0))
	require.NoError(t, acc0.SignTx(transferTx))

	b = bc.newBlock(initTx, transferTx)
	require.NoError(t, bc.AddBlock(b))
	t.Logf("recieveRublesTx: %v", transferTx.Hash().StringLE())

	transferTx = newNEP5Transfer(sh, priv0.GetScriptHash(), priv1.GetScriptHash(), 123)
	transferTx.Nonce = getNextNonce()
	transferTx.ValidUntilBlock = validUntilBlock
	transferTx.Signers = []transaction.Signer{
		{
			Account:          priv0ScriptHash,
			Scopes:           transaction.CalledByEntry,
			AllowedContracts: nil,
			AllowedGroups:    nil,
		},
	}
	require.NoError(t, addNetworkFee(bc, transferTx, acc0))
	require.NoError(t, acc0.SignTx(transferTx))

	b = bc.newBlock(transferTx)
	require.NoError(t, bc.AddBlock(b))
	t.Logf("sendRublesTx: %v", transferTx.Hash().StringLE())

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

	// Prepare some transaction for future submission.
	txSendRaw := newNEP5Transfer(neoHash, priv0ScriptHash, priv1.GetScriptHash(), int64(util.Fixed8FromInt64(1000)))
	txSendRaw.ValidUntilBlock = validUntilBlock
	txSendRaw.Nonce = getNextNonce()
	txSendRaw.Signers = []transaction.Signer{{
		Account:          priv0ScriptHash,
		Scopes:           transaction.CalledByEntry,
		AllowedContracts: nil,
		AllowedGroups:    nil,
	}}
	require.NoError(t, addNetworkFee(bc, txSendRaw, acc0))
	require.NoError(t, acc0.SignTx(txSendRaw))
	bw = io.NewBufBinWriter()
	txSendRaw.EncodeBinary(bw.BinWriter)
	t.Logf("sendrawtransaction: %s", hex.EncodeToString(bw.Bytes()))
}

func newNEP5Transfer(sc, from, to util.Uint160, amount int64) *transaction.Transaction {
	w := io.NewBufBinWriter()
	emit.AppCallWithOperationAndArgs(w.BinWriter, sc, "transfer", from, to, amount)
	emit.Opcode(w.BinWriter, opcode.ASSERT)

	script := w.Bytes()
	return transaction.New(testchain.Network(), script, 10000000)
}

func addSigners(txs ...*transaction.Transaction) {
	for _, tx := range txs {
		tx.Signers = []transaction.Signer{{
			Account:          neoOwner,
			Scopes:           transaction.CalledByEntry,
			AllowedContracts: nil,
			AllowedGroups:    nil,
		}}
	}
}

func signTx(bc *Blockchain, txs ...*transaction.Transaction) error {
	validators := bc.GetStandByValidators()
	rawScript, err := smartcontract.CreateDefaultMultiSigRedeemScript(validators)
	if err != nil {
		return fmt.Errorf("failed to sign tx: %w", err)
	}
	for _, tx := range txs {
		size := io.GetVarSize(tx)
		netFee, sizeDelta := CalculateNetworkFee(rawScript)
		tx.NetworkFee += netFee
		size += sizeDelta
		tx.NetworkFee += int64(size) * bc.FeePerByte()
		data := tx.GetSignedPart()
		tx.Scripts = []transaction.Witness{{
			InvocationScript:   testchain.Sign(data),
			VerificationScript: rawScript,
		}}
	}
	return nil
}

func addNetworkFee(bc *Blockchain, tx *transaction.Transaction, sender *wallet.Account) error {
	size := io.GetVarSize(tx)
	netFee, sizeDelta := CalculateNetworkFee(sender.Contract.Script)
	tx.NetworkFee += netFee
	size += sizeDelta
	for _, cosigner := range tx.Signers {
		contract := bc.GetContractState(cosigner.Account)
		if contract != nil {
			netFee, sizeDelta = CalculateNetworkFee(contract.Script)
			tx.NetworkFee += netFee
			size += sizeDelta
		}
	}
	tx.NetworkFee += int64(size) * bc.FeePerByte()
	return nil
}
