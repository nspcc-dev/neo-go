package core

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/internal/testchain"
	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer"
	"github.com/nspcc-dev/neo-go/pkg/core/chaindump"
	"github.com/nspcc-dev/neo-go/pkg/core/fee"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/native/noderoles"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/fixedn"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// multisig address which possess all NEO
var neoOwner = testchain.MultisigScriptHash()

// newTestChain should be called before newBlock invocation to properly setup
// global state.
func newTestChain(t *testing.T) *Blockchain {
	return newTestChainWithCustomCfg(t, nil)
}

func newTestChainWithCustomCfg(t *testing.T, f func(*config.Config)) *Blockchain {
	return newTestChainWithCustomCfgAndStore(t, nil, f)
}

func newTestChainWithCustomCfgAndStore(t *testing.T, st storage.Store, f func(*config.Config)) *Blockchain {
	chain := initTestChain(t, st, f)
	go chain.Run()
	t.Cleanup(chain.Close)
	return chain
}

func initTestChain(t *testing.T, st storage.Store, f func(*config.Config)) *Blockchain {
	unitTestNetCfg, err := config.Load("../../config", testchain.Network())
	require.NoError(t, err)
	if f != nil {
		f(&unitTestNetCfg)
	}
	if st == nil {
		st = storage.NewMemoryStore()
	}
	chain, err := NewBlockchain(st, unitTestNetCfg.ProtocolConfiguration, zaptest.NewLogger(t))
	require.NoError(t, err)
	return chain
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

func TestBug1728(t *testing.T) {
	src := `package example
	import "github.com/nspcc-dev/neo-go/pkg/interop/runtime"
	func init() { if true { } else { } }
	func _deploy(_ interface{}, isUpdate bool) {
		runtime.Log("Deploy")
	}`
	b, di, err := compiler.CompileWithDebugInfo("foo", strings.NewReader(src))
	require.NoError(t, err)
	m, err := di.ConvertToManifest(&compiler.Options{Name: "TestContract"})
	require.NoError(t, err)
	nf, err := nef.NewFile(b)
	require.NoError(t, err)
	nf.CalculateChecksum()

	rawManifest, err := json.Marshal(m)
	require.NoError(t, err)
	rawNef, err := nf.Bytes()
	require.NoError(t, err)

	bc := newTestChain(t)

	aer, err := invokeContractMethod(bc, 10000000000,
		bc.contracts.Management.Hash, "deploy", rawNef, rawManifest)
	require.NoError(t, err)
	require.Equal(t, aer.VMState, vm.HaltState)
}

func getDecodedBlock(t *testing.T, i int) *block.Block {
	data, err := getBlockData(i)
	require.NoError(t, err)

	b, err := hex.DecodeString(data["raw"].(string))
	require.NoError(t, err)

	block := block.New(false)
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
		Header: block.Header{
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

	bc := newTestChain(t)
	initBasicChain(t, bc)

	if saveChain {
		outStream, err := os.Create(prefix + "testblocks.acc")
		require.NoError(t, err)
		t.Cleanup(func() {
			outStream.Close()
		})

		writer := io.NewBinWriterFromIO(outStream)
		writer.WriteU32LE(bc.BlockHeight())
		err = chaindump.Dump(bc, writer, 1, bc.BlockHeight())
		require.NoError(t, err)
	}

	priv0 := testchain.PrivateKeyByID(0)
	priv1 := testchain.PrivateKeyByID(1)
	priv0ScriptHash := priv0.GetScriptHash()
	acc0 := wallet.NewAccountFromPrivateKey(priv0)

	// Prepare some transaction for future submission.
	txSendRaw := newNEP17Transfer(bc.contracts.NEO.Hash, priv0ScriptHash, priv1.GetScriptHash(), int64(fixedn.Fixed8FromInt64(1000)))
	txSendRaw.ValidUntilBlock = transaction.MaxValidUntilBlockIncrement
	txSendRaw.Nonce = 0x1234
	txSendRaw.Signers = []transaction.Signer{{
		Account:          priv0ScriptHash,
		Scopes:           transaction.CalledByEntry,
		AllowedContracts: nil,
		AllowedGroups:    nil,
	}}
	require.NoError(t, addNetworkFee(bc, txSendRaw, acc0))
	require.NoError(t, acc0.SignTx(testchain.Network(), txSendRaw))
	bw := io.NewBufBinWriter()
	txSendRaw.EncodeBinary(bw.BinWriter)
	t.Logf("sendrawtransaction: %s", base64.StdEncoding.EncodeToString(bw.Bytes()))
	require.False(t, saveChain)
}

func initBasicChain(t *testing.T, bc *Blockchain) {
	const prefix = "../rpc/server/testdata/"
	// Increase in case if you need more blocks
	const validUntilBlock = 1200

	// To be incremented after each created transaction to keep chain constant.
	var testNonce uint32 = 1

	// Use as nonce when new transaction is created to avoid random data in tests.
	getNextNonce := func() uint32 {
		testNonce++
		return testNonce
	}

	const neoAmount = 99999000

	gasHash := bc.contracts.GAS.Hash
	neoHash := bc.contracts.NEO.Hash
	policyHash := bc.contracts.Policy.Hash
	notaryHash := bc.contracts.Notary.Hash
	t.Logf("native GAS hash: %v", gasHash)
	t.Logf("native NEO hash: %v", neoHash)
	t.Logf("native Policy hash: %v", policyHash)
	t.Logf("native Notary hash: %v", notaryHash)
	t.Logf("Block0 hash: %s", bc.GetHeaderHash(0).StringLE())

	priv0 := testchain.PrivateKeyByID(0)
	priv0ScriptHash := priv0.GetScriptHash()

	require.Equal(t, big.NewInt(5000_0000), bc.GetUtilityTokenBalance(priv0ScriptHash)) // gas bounty
	// Move some NEO to one simple account.
	txMoveNeo, err := testchain.NewTransferFromOwner(bc, neoHash, priv0ScriptHash, neoAmount, getNextNonce(), validUntilBlock)
	require.NoError(t, err)
	// Move some GAS to one simple account.
	txMoveGas, err := testchain.NewTransferFromOwner(bc, gasHash, priv0ScriptHash, int64(fixedn.Fixed8FromInt64(1000)),
		getNextNonce(), validUntilBlock)
	require.NoError(t, err)

	b := bc.newBlock(txMoveNeo, txMoveGas)
	require.NoError(t, bc.AddBlock(b))
	checkTxHalt(t, bc, txMoveGas.Hash())
	checkTxHalt(t, bc, txMoveNeo.Hash())
	t.Logf("Block1 hash: %s", b.Hash().StringLE())
	bw := io.NewBufBinWriter()
	b.EncodeBinary(bw.BinWriter)
	require.NoError(t, bw.Err)
	jsonB, err := b.MarshalJSON()
	require.NoError(t, err)
	t.Logf("Block1 base64: %s", base64.StdEncoding.EncodeToString(bw.Bytes()))
	t.Logf("Block1 JSON: %s", string(jsonB))
	bw.Reset()
	b.Header.EncodeBinary(bw.BinWriter)
	require.NoError(t, bw.Err)
	jsonH, err := b.Header.MarshalJSON()
	require.NoError(t, err)
	t.Logf("Header1 base64: %s", base64.StdEncoding.EncodeToString(bw.Bytes()))
	t.Logf("Header1 JSON: %s", string(jsonH))
	jsonTxMoveNeo, err := txMoveNeo.MarshalJSON()
	require.NoError(t, err)
	t.Logf("txMoveNeo hash: %s", txMoveNeo.Hash().StringLE())
	t.Logf("txMoveNeo JSON: %s", string(jsonTxMoveNeo))
	t.Logf("txMoveNeo base64: %s", base64.StdEncoding.EncodeToString(txMoveNeo.Bytes()))
	t.Logf("txMoveGas hash: %s", txMoveGas.Hash().StringLE())

	require.True(t, bc.GetUtilityTokenBalance(priv0ScriptHash).Cmp(big.NewInt(1000*native.GASFactor)) >= 0)
	// info for getblockheader rpc tests
	t.Logf("header hash: %s", b.Hash().StringLE())
	buf := io.NewBufBinWriter()
	b.Header.EncodeBinary(buf.BinWriter)
	t.Logf("header: %s", hex.EncodeToString(buf.Bytes()))

	acc0 := wallet.NewAccountFromPrivateKey(priv0)

	// Push some contract into the chain.
	cfgPath := prefix + "test_contract.yml"
	txDeploy, cHash := newDeployTx(t, bc, priv0ScriptHash, prefix+"test_contract.go", "Rubl", &cfgPath)
	txDeploy.Nonce = getNextNonce()
	txDeploy.ValidUntilBlock = validUntilBlock
	require.NoError(t, addNetworkFee(bc, txDeploy, acc0))
	require.NoError(t, acc0.SignTx(testchain.Network(), txDeploy))
	b = bc.newBlock(txDeploy)
	require.NoError(t, bc.AddBlock(b))
	checkTxHalt(t, bc, txDeploy.Hash())
	t.Logf("txDeploy: %s", txDeploy.Hash().StringLE())
	t.Logf("Block2 hash: %s", b.Hash().StringLE())

	// Now invoke this contract.
	script := io.NewBufBinWriter()
	emit.AppCall(script.BinWriter, cHash, "putValue", callflag.All, "testkey", "testvalue")

	txInv := transaction.New(script.Bytes(), 1*native.GASFactor)
	txInv.Nonce = getNextNonce()
	txInv.ValidUntilBlock = validUntilBlock
	txInv.Signers = []transaction.Signer{{Account: priv0ScriptHash}}
	require.NoError(t, addNetworkFee(bc, txInv, acc0))
	require.NoError(t, acc0.SignTx(testchain.Network(), txInv))
	b = bc.newBlock(txInv)
	require.NoError(t, bc.AddBlock(b))
	checkTxHalt(t, bc, txInv.Hash())
	t.Logf("txInv: %s", txInv.Hash().StringLE())

	priv1 := testchain.PrivateKeyByID(1)
	txNeo0to1 := newNEP17Transfer(neoHash, priv0ScriptHash, priv1.GetScriptHash(), 1000)
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
	require.NoError(t, acc0.SignTx(testchain.Network(), txNeo0to1))
	b = bc.newBlock(txNeo0to1)
	require.NoError(t, bc.AddBlock(b))
	checkTxHalt(t, bc, txNeo0to1.Hash())

	w := io.NewBufBinWriter()
	emit.AppCall(w.BinWriter, cHash, "init", callflag.All)
	initTx := transaction.New(w.Bytes(), 1*native.GASFactor)
	initTx.Nonce = getNextNonce()
	initTx.ValidUntilBlock = validUntilBlock
	initTx.Signers = []transaction.Signer{{Account: priv0ScriptHash}}
	require.NoError(t, addNetworkFee(bc, initTx, acc0))
	require.NoError(t, acc0.SignTx(testchain.Network(), initTx))
	transferTx := newNEP17Transfer(cHash, cHash, priv0.GetScriptHash(), 1000)
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
	require.NoError(t, acc0.SignTx(testchain.Network(), transferTx))

	b = bc.newBlock(initTx, transferTx)
	require.NoError(t, bc.AddBlock(b))
	checkTxHalt(t, bc, initTx.Hash())
	checkTxHalt(t, bc, transferTx.Hash())
	t.Logf("recieveRublesTx: %v", transferTx.Hash().StringLE())

	transferTx = newNEP17Transfer(cHash, priv0.GetScriptHash(), priv1.GetScriptHash(), 123)
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
	require.NoError(t, acc0.SignTx(testchain.Network(), transferTx))

	b = bc.newBlock(transferTx)
	require.NoError(t, bc.AddBlock(b))
	checkTxHalt(t, bc, transferTx.Hash())
	t.Logf("sendRublesTx: %v", transferTx.Hash().StringLE())

	// Push verification contract into the chain.
	txDeploy2, _ := newDeployTx(t, bc, priv0ScriptHash, prefix+"verification_contract.go", "Verify", nil)
	txDeploy2.Nonce = getNextNonce()
	txDeploy2.ValidUntilBlock = validUntilBlock
	require.NoError(t, addNetworkFee(bc, txDeploy2, acc0))
	require.NoError(t, acc0.SignTx(testchain.Network(), txDeploy2))
	b = bc.newBlock(txDeploy2)
	require.NoError(t, bc.AddBlock(b))
	checkTxHalt(t, bc, txDeploy2.Hash())

	// Deposit some GAS to notary contract for priv0
	transferTx = newNEP17Transfer(gasHash, priv0.GetScriptHash(), notaryHash, 10_0000_0000, priv0.GetScriptHash(), int64(bc.BlockHeight()+1000))
	transferTx.Nonce = getNextNonce()
	transferTx.ValidUntilBlock = validUntilBlock
	transferTx.Signers = []transaction.Signer{
		{
			Account: priv0ScriptHash,
			Scopes:  transaction.CalledByEntry,
		},
	}
	require.NoError(t, addNetworkFee(bc, transferTx, acc0))
	transferTx.SystemFee += 10_0000
	require.NoError(t, acc0.SignTx(testchain.Network(), transferTx))

	b = bc.newBlock(transferTx)
	require.NoError(t, bc.AddBlock(b))
	checkTxHalt(t, bc, transferTx.Hash())
	t.Logf("notaryDepositTxPriv0: %v", transferTx.Hash().StringLE())

	// Designate new Notary node
	ntr, err := wallet.NewWalletFromFile(path.Join(notaryModulePath, "./testdata/notary1.json"))
	require.NoError(t, err)
	require.NoError(t, ntr.Accounts[0].Decrypt("one"))
	bc.setNodesByRole(t, true, noderoles.P2PNotary, keys.PublicKeys{ntr.Accounts[0].PrivateKey().PublicKey()})
	t.Logf("Designated Notary node: %s", hex.EncodeToString(ntr.Accounts[0].PrivateKey().PublicKey().Bytes()))

	// Push verification contract with arguments into the chain.
	txDeploy3, _ := newDeployTx(t, bc, priv0ScriptHash, prefix+"verification_with_args_contract.go", "VerifyWithArgs", nil)
	txDeploy3.Nonce = getNextNonce()
	txDeploy3.ValidUntilBlock = validUntilBlock
	require.NoError(t, addNetworkFee(bc, txDeploy3, acc0))
	require.NoError(t, acc0.SignTx(testchain.Network(), txDeploy3))
	b = bc.newBlock(txDeploy3)
	require.NoError(t, bc.AddBlock(b))
	checkTxHalt(t, bc, txDeploy3.Hash())

	// Compile contract to test `invokescript` RPC call
	_, _ = newDeployTx(t, bc, priv0ScriptHash, prefix+"invokescript_contract.go", "ContractForInvokescriptTest", nil)
}

func newNEP17Transfer(sc, from, to util.Uint160, amount int64, additionalArgs ...interface{}) *transaction.Transaction {
	w := io.NewBufBinWriter()
	emit.AppCall(w.BinWriter, sc, "transfer", callflag.All, from, to, amount, additionalArgs)
	emit.Opcodes(w.BinWriter, opcode.ASSERT)
	if w.Err != nil {
		panic(fmt.Errorf("failed to create nep17 transfer transaction: %w", w.Err))
	}

	script := w.Bytes()
	return transaction.New(script, 11000000)
}

func newDeployTx(t *testing.T, bc *Blockchain, sender util.Uint160, name, ctrName string, cfgName *string) (*transaction.Transaction, util.Uint160) {
	c, err := ioutil.ReadFile(name)
	require.NoError(t, err)
	tx, h, avm, err := testchain.NewDeployTx(bc, ctrName, sender, bytes.NewReader(c), cfgName)
	require.NoError(t, err)
	t.Logf("contract (%s): \n\tHash: %s\n\tAVM: %s", name, h.StringLE(), base64.StdEncoding.EncodeToString(avm))
	return tx, h
}

func addSigners(sender util.Uint160, txs ...*transaction.Transaction) {
	for _, tx := range txs {
		tx.Signers = []transaction.Signer{{
			Account:          sender,
			Scopes:           transaction.CalledByEntry,
			AllowedContracts: nil,
			AllowedGroups:    nil,
		}}
	}
}

func addNetworkFee(bc *Blockchain, tx *transaction.Transaction, sender *wallet.Account) error {
	size := io.GetVarSize(tx)
	netFee, sizeDelta := fee.Calculate(bc.GetBaseExecFee(), sender.Contract.Script)
	tx.NetworkFee += netFee
	size += sizeDelta
	for _, cosigner := range tx.Signers {
		contract := bc.GetContractState(cosigner.Account)
		if contract != nil {
			netFee, sizeDelta = fee.Calculate(bc.GetBaseExecFee(), contract.NEF.Script)
			tx.NetworkFee += netFee
			size += sizeDelta
		}
	}
	tx.NetworkFee += int64(size) * bc.FeePerByte()
	return nil
}

// Signer can be either bool or *wallet.Account.
// In the first case `true` means sign by committee, `false` means sign by validators.
func prepareContractMethodInvokeGeneric(chain *Blockchain, sysfee int64,
	hash util.Uint160, method string, signer interface{}, args ...interface{}) (*transaction.Transaction, error) {
	w := io.NewBufBinWriter()
	emit.AppCall(w.BinWriter, hash, method, callflag.All, args...)
	if w.Err != nil {
		return nil, w.Err
	}
	script := w.Bytes()
	tx := transaction.New(script, sysfee)
	tx.ValidUntilBlock = chain.blockHeight + 1
	var err error
	switch s := signer.(type) {
	case bool:
		if s {
			addSigners(testchain.CommitteeScriptHash(), tx)
			err = testchain.SignTxCommittee(chain, tx)
		} else {
			addSigners(neoOwner, tx)
			err = testchain.SignTx(chain, tx)
		}
	case *wallet.Account:
		signTxWithAccounts(chain, tx, s)
	case []*wallet.Account:
		signTxWithAccounts(chain, tx, s...)
	default:
		panic("invalid signer")
	}
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func signTxWithAccounts(chain *Blockchain, tx *transaction.Transaction, accs ...*wallet.Account) {
	scope := transaction.CalledByEntry
	for _, acc := range accs {
		tx.Signers = append(tx.Signers, transaction.Signer{
			Account: acc.PrivateKey().GetScriptHash(),
			Scopes:  scope,
		})
		scope = transaction.Global
	}
	size := io.GetVarSize(tx)
	for _, acc := range accs {
		netFee, sizeDelta := fee.Calculate(chain.GetBaseExecFee(), acc.Contract.Script)
		size += sizeDelta
		tx.NetworkFee += netFee
	}
	tx.NetworkFee += int64(size) * chain.FeePerByte()

	for _, acc := range accs {
		if err := acc.SignTx(testchain.Network(), tx); err != nil {
			panic(err)
		}
	}
}

func prepareContractMethodInvoke(chain *Blockchain, sysfee int64,
	hash util.Uint160, method string, args ...interface{}) (*transaction.Transaction, error) {
	return prepareContractMethodInvokeGeneric(chain, sysfee, hash,
		method, false, args...)
}

func persistBlock(chain *Blockchain, txs ...*transaction.Transaction) ([]*state.AppExecResult, error) {
	b := chain.newBlock(txs...)
	err := chain.AddBlock(b)
	if err != nil {
		return nil, err
	}

	aers := make([]*state.AppExecResult, len(txs))
	for i, tx := range txs {
		res, err := chain.GetAppExecResults(tx.Hash(), trigger.Application)
		if err != nil {
			return nil, err
		}
		aers[i] = &res[0]
	}
	return aers, nil
}

func invokeContractMethod(chain *Blockchain, sysfee int64, hash util.Uint160, method string, args ...interface{}) (*state.AppExecResult, error) {
	return invokeContractMethodGeneric(chain, sysfee, hash, method, false, args...)
}

func invokeContractMethodGeneric(chain *Blockchain, sysfee int64, hash util.Uint160, method string,
	signer interface{}, args ...interface{}) (*state.AppExecResult, error) {
	tx, err := prepareContractMethodInvokeGeneric(chain, sysfee, hash,
		method, signer, args...)
	if err != nil {
		return nil, err
	}
	aers, err := persistBlock(chain, tx)
	if err != nil {
		return nil, err
	}
	return aers[0], nil
}

func invokeContractMethodBy(t *testing.T, chain *Blockchain, signer *wallet.Account, hash util.Uint160, method string, args ...interface{}) (*state.AppExecResult, error) {
	var (
		netfee int64 = 1000_0000
		sysfee int64 = 1_0000_0000
	)
	transferTx := transferTokenFromMultisigAccount(t, chain, signer.PrivateKey().PublicKey().GetScriptHash(), chain.contracts.GAS.Hash, sysfee+netfee+1000_0000, nil)
	res, err := chain.GetAppExecResults(transferTx.Hash(), trigger.Application)
	require.NoError(t, err)
	require.Equal(t, vm.HaltState, res[0].VMState)
	require.Equal(t, 0, len(res[0].Stack))
	return invokeContractMethodGeneric(chain, sysfee, hash, method, signer, args...)
}

func transferTokenFromMultisigAccountCheckOK(t *testing.T, chain *Blockchain, to, tokenHash util.Uint160, amount int64, additionalArgs ...interface{}) {
	transferTx := transferTokenFromMultisigAccount(t, chain, to, tokenHash, amount, additionalArgs...)
	res, err := chain.GetAppExecResults(transferTx.Hash(), trigger.Application)
	require.NoError(t, err)
	require.Equal(t, vm.HaltState, res[0].VMState)
	require.Equal(t, 0, len(res[0].Stack))
}

func transferTokenFromMultisigAccount(t *testing.T, chain *Blockchain, to, tokenHash util.Uint160, amount int64, additionalArgs ...interface{}) *transaction.Transaction {
	transferTx := newNEP17Transfer(tokenHash, testchain.MultisigScriptHash(), to, amount, additionalArgs...)
	transferTx.SystemFee = 100000000
	transferTx.ValidUntilBlock = chain.BlockHeight() + 1
	addSigners(neoOwner, transferTx)
	require.NoError(t, testchain.SignTx(chain, transferTx))
	b := chain.newBlock(transferTx)
	require.NoError(t, chain.AddBlock(b))
	return transferTx
}

func checkResult(t *testing.T, result *state.AppExecResult, expected stackitem.Item) {
	require.Equal(t, vm.HaltState, result.VMState, result.FaultException)
	require.Equal(t, 1, len(result.Stack))
	require.Equal(t, expected, result.Stack[0])
}

func checkFAULTState(t *testing.T, result *state.AppExecResult) {
	require.Equal(t, vm.FaultState, result.VMState)
}

func checkBalanceOf(t *testing.T, chain *Blockchain, addr util.Uint160, expected int) {
	balance := chain.GetNEP17Balances(addr).Trackers[chain.contracts.GAS.ID]
	require.Equal(t, int64(expected), balance.Balance.Int64())
}

type NotaryFeerStub struct {
	bc blockchainer.Blockchainer
}

func (f NotaryFeerStub) FeePerByte() int64 { return f.bc.FeePerByte() }
func (f NotaryFeerStub) GetUtilityTokenBalance(acc util.Uint160) *big.Int {
	return f.bc.GetNotaryBalance(acc)
}
func (f NotaryFeerStub) BlockHeight() uint32           { return f.bc.BlockHeight() }
func (f NotaryFeerStub) P2PSigExtensionsEnabled() bool { return f.bc.P2PSigExtensionsEnabled() }
func NewNotaryFeerStub(bc blockchainer.Blockchainer) NotaryFeerStub {
	return NotaryFeerStub{
		bc: bc,
	}
}
