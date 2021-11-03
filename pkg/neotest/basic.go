package neotest

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer"
	"github.com/nspcc-dev/neo-go/pkg/core/fee"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/emit"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/require"
)

// Executor is a wrapper over chain state.
type Executor struct {
	Chain         blockchainer.Blockchainer
	Validator     Signer
	Committee     Signer
	CommitteeHash util.Uint160
	Contracts     map[string]*Contract
}

// NewExecutor creates new executor instance from provided blockchain and committee.
func NewExecutor(t *testing.T, bc blockchainer.Blockchainer, validator, committee Signer) *Executor {
	checkMultiSigner(t, validator)
	checkMultiSigner(t, committee)

	return &Executor{
		Chain:         bc,
		Validator:     validator,
		Committee:     committee,
		CommitteeHash: committee.ScriptHash(),
		Contracts:     make(map[string]*Contract),
	}
}

// TopBlock returns block with the highest index.
func (e *Executor) TopBlock(t *testing.T) *block.Block {
	b, err := e.Chain.GetBlock(e.Chain.GetHeaderHash(int(e.Chain.BlockHeight())))
	require.NoError(t, err)
	return b
}

// NativeHash returns native contract hash by name.
func (e *Executor) NativeHash(t *testing.T, name string) util.Uint160 {
	h, err := e.Chain.GetNativeContractScriptHash(name)
	require.NoError(t, err)
	return h
}

// NewUnsignedTx creates new unsigned transaction which invokes method of contract with hash.
func (e *Executor) NewUnsignedTx(t *testing.T, hash util.Uint160, method string, args ...interface{}) *transaction.Transaction {
	w := io.NewBufBinWriter()
	emit.AppCall(w.BinWriter, hash, method, callflag.All, args...)
	require.NoError(t, w.Err)

	script := w.Bytes()
	tx := transaction.New(script, 0)
	tx.Nonce = nonce()
	tx.ValidUntilBlock = e.Chain.BlockHeight() + 1
	return tx
}

// NewTx creates new transaction which invokes contract method.
// Transaction is signed with signer.
func (e *Executor) NewTx(t *testing.T, signers []Signer,
	hash util.Uint160, method string, args ...interface{}) *transaction.Transaction {
	tx := e.NewUnsignedTx(t, hash, method, args...)
	return e.SignTx(t, tx, -1, signers...)
}

// SignTx signs a transaction using provided signers.
func (e *Executor) SignTx(t *testing.T, tx *transaction.Transaction, sysFee int64, signers ...Signer) *transaction.Transaction {
	for _, acc := range signers {
		tx.Signers = append(tx.Signers, transaction.Signer{
			Account: acc.ScriptHash(),
			Scopes:  transaction.Global,
		})
	}
	addNetworkFee(e.Chain, tx, signers...)
	addSystemFee(e.Chain, tx, sysFee)

	for _, acc := range signers {
		require.NoError(t, acc.SignTx(e.Chain.GetConfig().Magic, tx))
	}
	return tx
}

// NewAccount returns new signer holding 100.0 GAS. This method advances the chain
// by one block with a transfer transaction.
func (e *Executor) NewAccount(t *testing.T) Signer {
	acc, err := wallet.NewAccount()
	require.NoError(t, err)

	tx := e.NewTx(t, []Signer{e.Committee},
		e.NativeHash(t, nativenames.Gas), "transfer",
		e.Committee.ScriptHash(), acc.Contract.ScriptHash(), int64(100_0000_0000), nil)
	e.AddNewBlock(t, tx)
	e.CheckHalt(t, tx.Hash())
	return NewSingleSigner(acc)
}

// DeployContract compiles and deploys contract to bc.
// data is an optional argument to `_deploy`.
// Returns hash of the deploy transaction.
func (e *Executor) DeployContract(t *testing.T, c *Contract, data interface{}) util.Uint256 {
	tx := e.NewDeployTx(t, e.Chain, c, data)
	e.AddNewBlock(t, tx)
	e.CheckHalt(t, tx.Hash())
	return tx.Hash()
}

// CheckHalt checks that transaction persisted with HALT state.
func (e *Executor) CheckHalt(t *testing.T, h util.Uint256, stack ...stackitem.Item) *state.AppExecResult {
	aer, err := e.Chain.GetAppExecResults(h, trigger.Application)
	require.NoError(t, err)
	require.Equal(t, vm.HaltState, aer[0].VMState, aer[0].FaultException)
	if len(stack) != 0 {
		require.Equal(t, stack, aer[0].Stack)
	}
	return &aer[0]
}

// CheckFault checks that transaction persisted with FAULT state.
// Raised exception is also checked to contain s as a substring.
func (e *Executor) CheckFault(t *testing.T, h util.Uint256, s string) {
	aer, err := e.Chain.GetAppExecResults(h, trigger.Application)
	require.NoError(t, err)
	require.Equal(t, vm.FaultState, aer[0].VMState)
	require.True(t, strings.Contains(aer[0].FaultException, s),
		"expected: %s, got: %s", s, aer[0].FaultException)
}

// NewDeployTx returns new deployment tx for contract signed by committee.
func (e *Executor) NewDeployTx(t *testing.T, bc blockchainer.Blockchainer, c *Contract, data interface{}) *transaction.Transaction {
	rawManifest, err := json.Marshal(c.Manifest)
	require.NoError(t, err)

	neb, err := c.NEF.Bytes()
	require.NoError(t, err)

	buf := io.NewBufBinWriter()
	emit.AppCall(buf.BinWriter, bc.ManagementContractHash(), "deploy", callflag.All, neb, rawManifest, data)
	require.NoError(t, buf.Err)

	tx := transaction.New(buf.Bytes(), 100*native.GASFactor)
	tx.Nonce = nonce()
	tx.ValidUntilBlock = bc.BlockHeight() + 1
	tx.Signers = []transaction.Signer{{
		Account: e.Committee.ScriptHash(),
		Scopes:  transaction.Global,
	}}
	addNetworkFee(bc, tx, e.Committee)
	require.NoError(t, e.Committee.SignTx(netmode.UnitTestNet, tx))
	return tx
}

func addSystemFee(bc blockchainer.Blockchainer, tx *transaction.Transaction, sysFee int64) {
	if sysFee >= 0 {
		tx.SystemFee = sysFee
		return
	}
	v, _ := TestInvoke(bc, tx) // ignore error to support failing transactions
	tx.SystemFee = v.GasConsumed()
}

func addNetworkFee(bc blockchainer.Blockchainer, tx *transaction.Transaction, signers ...Signer) {
	baseFee := bc.GetPolicer().GetBaseExecFee()
	size := io.GetVarSize(tx)
	for _, sgr := range signers {
		netFee, sizeDelta := fee.Calculate(baseFee, sgr.Script())
		tx.NetworkFee += netFee
		size += sizeDelta
	}
	tx.NetworkFee += int64(size) * bc.FeePerByte()
}

// NewUnsignedBlock creates new unsigned block from txs.
func (e *Executor) NewUnsignedBlock(t *testing.T, txs ...*transaction.Transaction) *block.Block {
	lastBlock := e.TopBlock(t)
	b := &block.Block{
		Header: block.Header{
			NextConsensus: e.Validator.ScriptHash(),
			Script: transaction.Witness{
				VerificationScript: e.Validator.Script(),
			},
			Timestamp: lastBlock.Timestamp + 1,
		},
		Transactions: txs,
	}
	if e.Chain.GetConfig().StateRootInHeader {
		b.StateRootEnabled = true
		b.PrevStateRoot = e.Chain.GetStateModule().CurrentLocalStateRoot()
	}
	b.PrevHash = lastBlock.Hash()
	b.Index = e.Chain.BlockHeight() + 1
	b.RebuildMerkleRoot()
	return b
}

// AddNewBlock creates a new block from provided transactions and adds it on bc.
func (e *Executor) AddNewBlock(t *testing.T, txs ...*transaction.Transaction) *block.Block {
	b := e.NewUnsignedBlock(t, txs...)
	e.SignBlock(b)
	require.NoError(t, e.Chain.AddBlock(b))
	return b
}

// SignBlock add validators signature to b.
func (e *Executor) SignBlock(b *block.Block) *block.Block {
	invoc := e.Validator.SignHashable(uint32(e.Chain.GetConfig().Magic), b)
	b.Script.InvocationScript = invoc
	return b
}

// AddBlockCheckHalt is a convenient wrapper over AddBlock and CheckHalt.
func (e *Executor) AddBlockCheckHalt(t *testing.T, txs ...*transaction.Transaction) *block.Block {
	b := e.AddNewBlock(t, txs...)
	for _, tx := range txs {
		e.CheckHalt(t, tx.Hash())
	}
	return b
}

// TestInvoke creates a test VM with dummy block and executes transaction in it.
func TestInvoke(bc blockchainer.Blockchainer, tx *transaction.Transaction) (*vm.VM, error) {
	lastBlock, err := bc.GetBlock(bc.GetHeaderHash(int(bc.BlockHeight())))
	if err != nil {
		return nil, err
	}
	b := &block.Block{
		Header: block.Header{
			Index:     bc.BlockHeight() + 1,
			Timestamp: lastBlock.Timestamp + 1,
		},
	}

	// `GetTestVM` as well as `Run` can use transaction hash which will set cached value.
	// This is unwanted behaviour so we explicitly copy transaction to perform execution.
	ttx := *tx
	v, f := bc.GetTestVM(trigger.Application, &ttx, b)
	defer f()

	v.LoadWithFlags(tx.Script, callflag.All)
	err = v.Run()
	return v, err
}
