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
	"github.com/nspcc-dev/neo-go/pkg/internal/testchain"
	"github.com/nspcc-dev/neo-go/pkg/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// multisig address which possess all NEO
var neoOwner = testchain.MultisigScriptHash()

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
			transaction.New([]byte{byte(opcode.PUSH1)}, 0),
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
	t.Logf("native GAS hash: %v", gasHash)
	t.Logf("native NEO hash: %v", neoHash)

	priv0 := testchain.PrivateKeyByID(0)
	priv0ScriptHash := priv0.GetScriptHash()

	require.Equal(t, util.Fixed8FromInt64(0), bc.GetUtilityTokenBalance(priv0ScriptHash))
	// Move some NEO to one simple account.
	txMoveNeo := newNEP5Transfer(neoHash, neoOwner, priv0ScriptHash, neoAmount)
	txMoveNeo.ValidUntilBlock = validUntilBlock
	txMoveNeo.Nonce = getNextNonce()
	txMoveNeo.Sender = neoOwner
	txMoveNeo.Cosigners = []transaction.Cosigner{{
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
	txMoveGas.Sender = neoOwner
	txMoveGas.Cosigners = []transaction.Cosigner{{
		Account:          neoOwner,
		Scopes:           transaction.CalledByEntry,
		AllowedContracts: nil,
		AllowedGroups:    nil,
	}}
	require.NoError(t, signTx(bc, txMoveGas))
	b := bc.newBlock(txMoveNeo, txMoveGas)
	require.NoError(t, bc.AddBlock(b))
	t.Logf("txMoveNeo: %s", txMoveNeo.Hash().StringLE())
	t.Logf("txMoveGas: %s", txMoveGas.Hash().StringLE())

	require.Equal(t, util.Fixed8FromInt64(1000), bc.GetUtilityTokenBalance(priv0ScriptHash))
	// info for getblockheader rpc tests
	t.Logf("header hash: %s", b.Hash().StringLE())
	buf := io.NewBufBinWriter()
	b.Header().EncodeBinary(buf.BinWriter)
	t.Logf("header: %s", hex.EncodeToString(buf.Bytes()))

	acc0, err := wallet.NewAccountFromWIF(priv0.WIF())
	require.NoError(t, err)

	// Push some contract into the chain.
	avm, err := ioutil.ReadFile(prefix + "test_contract.avm")
	require.NoError(t, err)
	t.Logf("contractHash: %s", hash.Hash160(avm).StringLE())

	script := io.NewBufBinWriter()
	m := manifest.NewManifest(hash.Hash160(avm))
	m.ABI.EntryPoint.Name = "Main"
	m.ABI.EntryPoint.Parameters = []manifest.Parameter{
		manifest.NewParameter("method", smartcontract.StringType),
		manifest.NewParameter("params", smartcontract.ArrayType),
	}
	m.ABI.EntryPoint.ReturnType = smartcontract.BoolType
	m.Features = smartcontract.HasStorage
	bs, err := testserdes.EncodeBinary(m)
	require.NoError(t, err)
	emit.Bytes(script.BinWriter, bs)
	emit.Bytes(script.BinWriter, avm)
	emit.Syscall(script.BinWriter, "Neo.Contract.Create")
	txScript := script.Bytes()

	invFee := util.Fixed8FromFloat(100)
	txDeploy := transaction.New(txScript, invFee)
	txDeploy.Nonce = getNextNonce()
	txDeploy.ValidUntilBlock = validUntilBlock
	txDeploy.Sender = priv0ScriptHash
	require.NoError(t, addNetworkFee(bc, txDeploy, acc0))
	require.NoError(t, acc0.SignTx(txDeploy))
	b = bc.newBlock(txDeploy)
	require.NoError(t, bc.AddBlock(b))
	t.Logf("txDeploy: %s", txDeploy.Hash().StringLE())

	// Now invoke this contract.
	script = io.NewBufBinWriter()
	emit.AppCallWithOperationAndArgs(script.BinWriter, hash.Hash160(avm), "Put", "testkey", "testvalue")

	txInv := transaction.New(script.Bytes(), 0)
	txInv.Nonce = getNextNonce()
	txInv.ValidUntilBlock = validUntilBlock
	txInv.Sender = priv0ScriptHash
	require.NoError(t, addNetworkFee(bc, txInv, acc0))
	require.NoError(t, acc0.SignTx(txInv))
	b = bc.newBlock(txInv)
	require.NoError(t, bc.AddBlock(b))
	t.Logf("txInv: %s", txInv.Hash().StringLE())

	priv1 := testchain.PrivateKeyByID(1)
	txNeo0to1 := newNEP5Transfer(neoHash, priv0ScriptHash, priv1.GetScriptHash(), 1000)
	txNeo0to1.Nonce = getNextNonce()
	txNeo0to1.ValidUntilBlock = validUntilBlock
	txNeo0to1.Sender = priv0ScriptHash
	txNeo0to1.Cosigners = []transaction.Cosigner{
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
	initTx := transaction.New(w.Bytes(), 0)
	initTx.Nonce = getNextNonce()
	initTx.ValidUntilBlock = validUntilBlock
	initTx.Sender = priv0ScriptHash
	require.NoError(t, addNetworkFee(bc, initTx, acc0))
	require.NoError(t, acc0.SignTx(initTx))
	transferTx := newNEP5Transfer(sh, sh, priv0.GetScriptHash(), 1000)
	transferTx.Nonce = getNextNonce()
	transferTx.ValidUntilBlock = validUntilBlock
	transferTx.Sender = priv0ScriptHash
	transferTx.Cosigners = []transaction.Cosigner{
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
	transferTx.Sender = priv0ScriptHash
	transferTx.Cosigners = []transaction.Cosigner{
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
	txSendRaw.Sender = priv0ScriptHash
	txSendRaw.Cosigners = []transaction.Cosigner{{
		Account:          priv0ScriptHash,
		Scopes:           transaction.CalledByEntry,
		AllowedContracts: nil,
		AllowedGroups:    nil,
	}}
	require.NoError(t, addNetworkFee(bc, txSendRaw, acc0))
	require.NoError(t, acc0.SignTx(txSendRaw))
	bw := io.NewBufBinWriter()
	txSendRaw.EncodeBinary(bw.BinWriter)
	t.Logf("sendrawtransaction: %s", hex.EncodeToString(bw.Bytes()))
}

func newNEP5Transfer(sc, from, to util.Uint160, amount int64) *transaction.Transaction {
	w := io.NewBufBinWriter()
	emit.AppCallWithOperationAndArgs(w.BinWriter, sc, "transfer", from, to, amount)
	emit.Opcode(w.BinWriter, opcode.ASSERT)

	script := w.Bytes()
	return transaction.New(script, 0)
}

func addSender(txs ...*transaction.Transaction) error {
	for _, tx := range txs {
		tx.Sender = neoOwner
	}
	return nil
}

func signTx(bc *Blockchain, txs ...*transaction.Transaction) error {
	validators, err := getValidators(bc.config)
	if err != nil {
		return errors.Wrap(err, "fail to sign tx")
	}
	rawScript, err := smartcontract.CreateMultiSigRedeemScript(len(bc.config.StandbyValidators)/2+1, validators)
	if err != nil {
		return errors.Wrap(err, "fail to sign tx")
	}
	for _, tx := range txs {
		size := io.GetVarSize(tx)
		netFee, sizeDelta := CalculateNetworkFee(rawScript)
		tx.NetworkFee = tx.NetworkFee.Add(netFee)
		size += sizeDelta
		tx.NetworkFee = tx.NetworkFee.Add(util.Fixed8(int64(size) * int64(bc.FeePerByte())))
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
	for _, cosigner := range tx.Cosigners {
		contract := bc.GetContractState(cosigner.Account)
		if contract != nil {
			netFee, sizeDelta = CalculateNetworkFee(contract.Script)
			tx.NetworkFee += netFee
			size += sizeDelta
		}
	}
	tx.NetworkFee += util.Fixed8(int64(size) * int64(bc.FeePerByte()))
	return nil
}
