package core

import (
	"encoding/json"
	"fmt"
	"math/big"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/internal/testchain"
	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer"
	"github.com/nspcc-dev/neo-go/pkg/core/fee"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
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
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// multisig address which possess all NEO.
var neoOwner = testchain.MultisigScriptHash()

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
	dbPath := t.TempDir()
	dbOptions := storage.LevelDBOptions{
		DataDirectoryPath: dbPath,
	}
	newLevelStore, err := storage.NewLevelDBStore(dbOptions)
	require.Nil(t, err, "NewLevelDBStore error")
	return newLevelStore
}

func newBoltStoreForTesting(t testing.TB) storage.Store {
	d := t.TempDir()
	dbPath := filepath.Join(d, "test_bolt_db")
	boltDBStore, err := storage.NewBoltDBStore(storage.BoltDBOptions{FilePath: dbPath})
	require.NoError(t, err)
	return boltDBStore
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
