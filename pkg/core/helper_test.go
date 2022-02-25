package core

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/internal/testchain"
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
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/encoding/fixedn"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/rpc/client/nns"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// multisig address which possess all NEO.
var neoOwner = testchain.MultisigScriptHash()

// examplesPrefix is a prefix of the example smart-contracts.
const examplesPrefix = "../../examples/"

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

func newLevelDBForTesting(t testing.TB) storage.Store {
	newLevelStore, _ := newLevelDBForTestingWithPath(t, "")
	return newLevelStore
}

func newLevelDBForTestingWithPath(t testing.TB, dbPath string) (storage.Store, string) {
	if dbPath == "" {
		dbPath = t.TempDir()
	}
	dbOptions := storage.LevelDBOptions{
		DataDirectoryPath: dbPath,
	}
	newLevelStore, err := storage.NewLevelDBStore(dbOptions)
	require.Nil(t, err, "NewLevelDBStore error")
	return newLevelStore, dbPath
}

func newBoltStoreForTesting(t testing.TB) storage.Store {
	boltDBStore, _ := newBoltStoreForTestingWithPath(t, "")
	return boltDBStore
}

func newBoltStoreForTestingWithPath(t testing.TB, dbPath string) (storage.Store, string) {
	if dbPath == "" {
		d := t.TempDir()
		dbPath = filepath.Join(d, "test_bolt_db")
	}
	boltDBStore, err := storage.NewBoltDBStore(storage.BoltDBOptions{FilePath: dbPath})
	require.NoError(t, err)
	return boltDBStore, dbPath
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

func TestBug1728(t *testing.T) {
	src := `package example
	import "github.com/nspcc-dev/neo-go/pkg/interop/runtime"
	func init() { if true { } else { } }
	func _deploy(_ interface{}, isUpdate bool) {
		runtime.Log("Deploy")
	}`
	nf, di, err := compiler.CompileWithOptions("foo.go", strings.NewReader(src), nil)
	require.NoError(t, err)
	m, err := di.ConvertToManifest(&compiler.Options{Name: "TestContract"})
	require.NoError(t, err)

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
	txSendRaw.ValidUntilBlock = bc.config.MaxValidUntilBlockIncrement
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
	t.Logf("sendrawtransaction: \n\tbase64: %s\n\tHash LE: %s", base64.StdEncoding.EncodeToString(bw.Bytes()), txSendRaw.Hash().StringLE())
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
	priv1 := testchain.PrivateKeyByID(1)
	priv1ScriptHash := priv1.GetScriptHash()
	acc0 := wallet.NewAccountFromPrivateKey(priv0)
	acc1 := wallet.NewAccountFromPrivateKey(priv1)

	deployContractFromPriv0 := func(t *testing.T, path, contractName string, configPath *string, expectedID int32) (util.Uint256, util.Uint256, util.Uint160) {
		txDeploy, _ := newDeployTx(t, bc, priv0ScriptHash, path, contractName, configPath)
		txDeploy.Nonce = getNextNonce()
		txDeploy.ValidUntilBlock = validUntilBlock
		require.NoError(t, addNetworkFee(bc, txDeploy, acc0))
		require.NoError(t, acc0.SignTx(testchain.Network(), txDeploy))
		b := bc.newBlock(txDeploy)
		require.NoError(t, bc.AddBlock(b)) // block #11
		checkTxHalt(t, bc, txDeploy.Hash())
		sh, err := bc.GetContractScriptHash(expectedID)
		require.NoError(t, err)
		return b.Hash(), txDeploy.Hash(), sh
	}

	require.Equal(t, big.NewInt(5000_0000), bc.GetUtilityTokenBalance(priv0ScriptHash)) // gas bounty

	// Block #1: move 1000 GAS and neoAmount NEO to priv0.
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

	// Block #2: deploy test_contract.
	cfgPath := prefix + "test_contract.yml"
	block2H, txDeployH, cHash := deployContractFromPriv0(t, prefix+"test_contract.go", "Rubl", &cfgPath, 1)
	t.Logf("txDeploy: %s", txDeployH.StringLE())
	t.Logf("Block2 hash: %s", block2H.StringLE())

	// Block #3: invoke `putValue` method on the test_contract.
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

	// Block #4: transfer 0.0000_1 NEO from priv0 to priv1.
	txNeo0to1 := newNEP17Transfer(neoHash, priv0ScriptHash, priv1ScriptHash, 1000)
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

	// Block #5: initialize rubles contract and transfer 1000 rubles from the contract to priv0.
	w := io.NewBufBinWriter()
	emit.AppCall(w.BinWriter, cHash, "init", callflag.All)
	initTx := transaction.New(w.Bytes(), 1*native.GASFactor)
	initTx.Nonce = getNextNonce()
	initTx.ValidUntilBlock = validUntilBlock
	initTx.Signers = []transaction.Signer{{Account: priv0ScriptHash}}
	require.NoError(t, addNetworkFee(bc, initTx, acc0))
	require.NoError(t, acc0.SignTx(testchain.Network(), initTx))
	transferTx := newNEP17Transfer(cHash, cHash, priv0ScriptHash, 1000)
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
	transferTx.SystemFee += 1000000
	require.NoError(t, acc0.SignTx(testchain.Network(), transferTx))
	b = bc.newBlock(initTx, transferTx)
	require.NoError(t, bc.AddBlock(b))
	checkTxHalt(t, bc, initTx.Hash())
	checkTxHalt(t, bc, transferTx.Hash())
	t.Logf("recieveRublesTx: %v", transferTx.Hash().StringLE())

	// Block #6: transfer 123 rubles from priv0 to priv1
	transferTx = newNEP17Transfer(cHash, priv0.GetScriptHash(), priv1ScriptHash, 123)
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
	transferTx.SystemFee += 1000000
	require.NoError(t, acc0.SignTx(testchain.Network(), transferTx))
	b = bc.newBlock(transferTx)
	require.NoError(t, bc.AddBlock(b))
	checkTxHalt(t, bc, transferTx.Hash())
	t.Logf("sendRublesTx: %v", transferTx.Hash().StringLE())

	// Block #7: push verification contract into the chain.
	verifyPath := filepath.Join(prefix, "verify", "verification_contract.go")
	_, _, _ = deployContractFromPriv0(t, verifyPath, "Verify", nil, 2)

	// Block #8: deposit some GAS to notary contract for priv0.
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

	// Block #9: designate new Notary node.
	ntr, err := wallet.NewWalletFromFile(path.Join(notaryModulePath, "./testdata/notary1.json"))
	require.NoError(t, err)
	require.NoError(t, ntr.Accounts[0].Decrypt("one", ntr.Scrypt))
	bc.setNodesByRole(t, true, noderoles.P2PNotary, keys.PublicKeys{ntr.Accounts[0].PrivateKey().PublicKey()})
	t.Logf("Designated Notary node: %s", hex.EncodeToString(ntr.Accounts[0].PrivateKey().PublicKey().Bytes()))

	// Block #10: push verification contract with arguments into the chain.
	verifyPath = filepath.Join(prefix, "verify_args", "verification_with_args_contract.go")
	_, _, _ = deployContractFromPriv0(t, verifyPath, "VerifyWithArgs", nil, 3) // block #10

	// Block #11: push NameService contract into the chain.
	nsPath := examplesPrefix + "nft-nd-nns/"
	nsConfigPath := nsPath + "nns.yml"
	_, _, nsHash := deployContractFromPriv0(t, nsPath, nsPath, &nsConfigPath, 4) // block #11

	// Block #12: transfer funds to committee for futher NS record registration.
	transferFundsToCommittee(t, bc) // block #12

	// Block #13: add `.com` root to NNS.
	res, err := invokeContractMethodGeneric(bc, -1,
		nsHash, "addRoot", true, "com") // block #13
	require.NoError(t, err)
	checkResult(t, res, stackitem.Null{})

	// Block #14: register `neo.com` via NNS.
	res, err = invokeContractMethodGeneric(bc, -1,
		nsHash, "register", acc0, "neo.com", priv0ScriptHash) // block #14
	require.NoError(t, err)
	checkResult(t, res, stackitem.NewBool(true))
	require.Equal(t, 1, len(res.Events)) // transfer
	tokenID, err := res.Events[0].Item.Value().([]stackitem.Item)[3].TryBytes()
	require.NoError(t, err)
	t.Logf("NNS token #1 ID (hex): %s", hex.EncodeToString(tokenID))

	// Block #15: set A record type with priv0 owner via NNS.
	res, err = invokeContractMethodGeneric(bc, -1, nsHash,
		"setRecord", acc0, "neo.com", int64(nns.A), "1.2.3.4") // block #15
	require.NoError(t, err)
	checkResult(t, res, stackitem.Null{})

	// Block #16: invoke `test_contract.go`: put new value with the same key to check `getstate` RPC call
	script.Reset()
	emit.AppCall(script.BinWriter, cHash, "putValue", callflag.All, "testkey", "newtestvalue")
	// Invoke `test_contract.go`: put values to check `findstates` RPC call
	emit.AppCall(script.BinWriter, cHash, "putValue", callflag.All, "aa", "v1")
	emit.AppCall(script.BinWriter, cHash, "putValue", callflag.All, "aa10", "v2")
	emit.AppCall(script.BinWriter, cHash, "putValue", callflag.All, "aa50", "v3")
	txInv = transaction.New(script.Bytes(), 1*native.GASFactor)
	txInv.Nonce = getNextNonce()
	txInv.ValidUntilBlock = validUntilBlock
	txInv.Signers = []transaction.Signer{{Account: priv0ScriptHash}}
	require.NoError(t, addNetworkFee(bc, txInv, acc0))
	require.NoError(t, acc0.SignTx(testchain.Network(), txInv))
	b = bc.newBlock(txInv)
	require.NoError(t, bc.AddBlock(b)) // block #16
	checkTxHalt(t, bc, txInv.Hash())

	// Block #17: deploy NeoFS Object contract (NEP11-Divisible).
	nfsPath := examplesPrefix + "nft-d/"
	nfsConfigPath := nfsPath + "nft.yml"
	_, _, nfsHash := deployContractFromPriv0(t, nfsPath, nfsPath, &nfsConfigPath, 5) // block #17

	// Block #18: mint 1.00 NFSO token by transferring 10 GAS to NFSO contract.
	containerID := util.Uint256{1, 2, 3}
	objectID := util.Uint256{4, 5, 6}
	txGas0toNFS := newNEP17Transfer(gasHash, priv0ScriptHash, nfsHash, 10_0000_0000, containerID.BytesBE(), objectID.BytesBE())
	txGas0toNFS.SystemFee += 4000_0000
	txGas0toNFS.Nonce = getNextNonce()
	txGas0toNFS.ValidUntilBlock = validUntilBlock
	txGas0toNFS.Signers = []transaction.Signer{
		{
			Account: priv0ScriptHash,
			Scopes:  transaction.CalledByEntry,
		},
	}
	require.NoError(t, addNetworkFee(bc, txGas0toNFS, acc0))
	require.NoError(t, acc0.SignTx(testchain.Network(), txGas0toNFS))
	b = bc.newBlock(txGas0toNFS)
	require.NoError(t, bc.AddBlock(b)) // block #18
	checkTxHalt(t, bc, txGas0toNFS.Hash())
	aer, _ := bc.GetAppExecResults(txGas0toNFS.Hash(), trigger.Application)
	require.Equal(t, 2, len(aer[0].Events)) // GAS transfer + NFSO transfer
	tokenID, err = aer[0].Events[1].Item.Value().([]stackitem.Item)[3].TryBytes()
	require.NoError(t, err)
	t.Logf("NFSO token #1 ID (hex): %s", hex.EncodeToString(tokenID))

	// Block #19: transfer 0.25 NFSO from priv0 to priv1.
	script.Reset()
	emit.AppCall(script.BinWriter, nfsHash, "transfer", callflag.All, priv0ScriptHash, priv1ScriptHash, 25, tokenID, nil)
	emit.Opcodes(script.BinWriter, opcode.ASSERT)
	require.NoError(t, script.Err)
	txNFS0to1 := transaction.New(script.Bytes(), 1*native.GASFactor)
	txNFS0to1.Nonce = getNextNonce()
	txNFS0to1.ValidUntilBlock = validUntilBlock
	txNFS0to1.Signers = []transaction.Signer{{Account: priv0ScriptHash, Scopes: transaction.CalledByEntry}}
	require.NoError(t, addNetworkFee(bc, txNFS0to1, acc0))
	require.NoError(t, acc0.SignTx(testchain.Network(), txNFS0to1))
	b = bc.newBlock(txNFS0to1)
	require.NoError(t, bc.AddBlock(b)) // block #19
	checkTxHalt(t, bc, txNFS0to1.Hash())

	// Block #20: transfer 1000 GAS to priv1.
	txMoveGas, err = testchain.NewTransferFromOwner(bc, gasHash, priv1ScriptHash, int64(fixedn.Fixed8FromInt64(1000)),
		getNextNonce(), validUntilBlock)
	require.NoError(t, err)
	require.NoError(t, bc.AddBlock(bc.newBlock(txMoveGas)))
	checkTxHalt(t, bc, txMoveGas.Hash()) // block #20

	// Block #21: transfer 0.05 NFSO from priv1 back to priv0.
	script.Reset()
	emit.AppCall(script.BinWriter, nfsHash, "transfer", callflag.All, priv1ScriptHash, priv0.GetScriptHash(), 5, tokenID, nil)
	emit.Opcodes(script.BinWriter, opcode.ASSERT)
	require.NoError(t, script.Err)
	txNFS1to0 := transaction.New(script.Bytes(), 1*native.GASFactor)
	txNFS1to0.Nonce = getNextNonce()
	txNFS1to0.ValidUntilBlock = validUntilBlock
	txNFS1to0.Signers = []transaction.Signer{{Account: priv1ScriptHash, Scopes: transaction.CalledByEntry}}
	require.NoError(t, addNetworkFee(bc, txNFS1to0, acc0))
	require.NoError(t, acc1.SignTx(testchain.Network(), txNFS1to0))
	b = bc.newBlock(txNFS1to0)
	require.NoError(t, bc.AddBlock(b)) // block #21
	checkTxHalt(t, bc, txNFS1to0.Hash())

	// Compile contract to test `invokescript` RPC call
	invokePath := filepath.Join(prefix, "invoke", "invokescript_contract.go")
	invokeCfg := filepath.Join(prefix, "invoke", "invoke.yml")
	_, _ = newDeployTx(t, bc, priv0ScriptHash, invokePath, "ContractForInvokescriptTest", &invokeCfg)
}

func newNEP17Transfer(sc, from, to util.Uint160, amount int64, additionalArgs ...interface{}) *transaction.Transaction {
	return newNEP17TransferWithAssert(sc, from, to, amount, true, additionalArgs...)
}

func newNEP17TransferWithAssert(sc, from, to util.Uint160, amount int64, needAssert bool, additionalArgs ...interface{}) *transaction.Transaction {
	w := io.NewBufBinWriter()
	emit.AppCall(w.BinWriter, sc, "transfer", callflag.All, from, to, amount, additionalArgs)
	if needAssert {
		emit.Opcodes(w.BinWriter, opcode.ASSERT)
	}
	if w.Err != nil {
		panic(fmt.Errorf("failed to create NEP-17 transfer transaction: %w", w.Err))
	}

	script := w.Bytes()
	return transaction.New(script, 11000000)
}

func newDeployTx(t *testing.T, bc *Blockchain, sender util.Uint160, name, ctrName string, cfgName *string) (*transaction.Transaction, util.Uint160) {
	tx, h, avm, err := testchain.NewDeployTx(bc, name, sender, nil, cfgName)
	require.NoError(t, err)
	t.Logf("contract (%s): \n\tHash: %s\n\tAVM: %s", name, h.StringLE(), base64.StdEncoding.EncodeToString(avm))
	return tx, h
}

func addSigners(sender util.Uint160, txs ...*transaction.Transaction) {
	for _, tx := range txs {
		tx.Signers = []transaction.Signer{{
			Account:          sender,
			Scopes:           transaction.Global,
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
	tx := transaction.New(script, 0)
	tx.ValidUntilBlock = chain.blockHeight + 1
	var err error
	switch s := signer.(type) {
	case bool:
		if s {
			addSigners(testchain.CommitteeScriptHash(), tx)
			setTxSystemFee(chain, sysfee, tx)
			err = testchain.SignTxCommittee(chain, tx)
		} else {
			addSigners(neoOwner, tx)
			setTxSystemFee(chain, sysfee, tx)
			err = testchain.SignTx(chain, tx)
		}
	case *wallet.Account:
		signTxWithAccounts(chain, sysfee, tx, s)
	case []*wallet.Account:
		signTxWithAccounts(chain, sysfee, tx, s...)
	default:
		panic("invalid signer")
	}
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func setTxSystemFee(bc *Blockchain, sysFee int64, tx *transaction.Transaction) {
	if sysFee >= 0 {
		tx.SystemFee = sysFee
		return
	}

	lastBlock := bc.topBlock.Load().(*block.Block)
	b := &block.Block{
		Header: block.Header{
			Index:     lastBlock.Index + 1,
			Timestamp: lastBlock.Timestamp + 1000,
		},
		Transactions: []*transaction.Transaction{tx},
	}

	ttx := *tx // prevent setting 'hash' field
	ic := bc.GetTestVM(trigger.Application, &ttx, b)
	defer ic.Finalize()

	ic.VM.LoadWithFlags(tx.Script, callflag.All)
	_ = ic.VM.Run()
	tx.SystemFee = ic.VM.GasConsumed()
}

func signTxWithAccounts(chain *Blockchain, sysFee int64, tx *transaction.Transaction, accs ...*wallet.Account) {
	scope := transaction.CalledByEntry
	for _, acc := range accs {
		accH, _ := address.StringToUint160(acc.Address)
		tx.Signers = append(tx.Signers, transaction.Signer{
			Account: accH,
			Scopes:  scope,
		})
		scope = transaction.Global
	}
	setTxSystemFee(chain, sysFee, tx)
	size := io.GetVarSize(tx)
	for _, acc := range accs {
		if acc.Contract.Deployed {
			// don't need precise calculation for tests
			tx.NetworkFee += 1000_0000
			continue
		}
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

func transferTokenFromMultisigAccountCheckOK(t *testing.T, chain *Blockchain, to, tokenHash util.Uint160, amount int64, additionalArgs ...interface{}) {
	transferTx := transferTokenFromMultisigAccount(t, chain, to, tokenHash, amount, additionalArgs...)
	res, err := chain.GetAppExecResults(transferTx.Hash(), trigger.Application)
	require.NoError(t, err)
	require.Equal(t, vm.HaltState, res[0].VMState)
	require.Equal(t, 0, len(res[0].Stack))
}

func transferTokenFromMultisigAccount(t *testing.T, chain *Blockchain, to, tokenHash util.Uint160, amount int64, additionalArgs ...interface{}) *transaction.Transaction {
	return transferTokenFromMultisigAccountWithAssert(t, chain, to, tokenHash, amount, true, additionalArgs...)
}

func transferTokenFromMultisigAccountWithAssert(t *testing.T, chain *Blockchain, to, tokenHash util.Uint160, amount int64, needAssert bool, additionalArgs ...interface{}) *transaction.Transaction {
	transferTx := newNEP17TransferWithAssert(tokenHash, testchain.MultisigScriptHash(), to, amount, needAssert, additionalArgs...)
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

func checkTxHalt(t testing.TB, bc *Blockchain, h util.Uint256) {
	aer, err := bc.GetAppExecResults(h, trigger.Application)
	require.NoError(t, err)
	require.Equal(t, 1, len(aer))
	require.Equal(t, vm.HaltState, aer[0].VMState, aer[0].FaultException)
}

func checkFAULTState(t *testing.T, result *state.AppExecResult) {
	require.Equal(t, vm.FaultState, result.VMState)
}

func checkBalanceOf(t *testing.T, chain *Blockchain, addr util.Uint160, expected int) {
	balance := chain.GetUtilityTokenBalance(addr)
	require.Equal(t, int64(expected), balance.Int64())
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
