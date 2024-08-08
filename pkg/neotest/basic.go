package neotest

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/fee"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/nspcc-dev/neo-go/pkg/vm/vmstate"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/require"
)

// Executor is a wrapper over chain state.
type Executor struct {
	Chain           *core.Blockchain
	Validator       Signer
	Committee       Signer
	CommitteeHash   util.Uint160
	Contracts       map[string]*Contract
	collectCoverage bool
}

// NewExecutor creates a new executor instance from the provided blockchain and committee.
func NewExecutor(t testing.TB, bc *core.Blockchain, validator, committee Signer) *Executor {
	checkMultiSigner(t, validator)
	checkMultiSigner(t, committee)

	return &Executor{
		Chain:           bc,
		Validator:       validator,
		Committee:       committee,
		CommitteeHash:   committee.ScriptHash(),
		Contracts:       make(map[string]*Contract),
		collectCoverage: isCoverageEnabled(),
	}
}

// TopBlock returns the block with the highest index.
func (e *Executor) TopBlock(t testing.TB) *block.Block {
	b, err := e.Chain.GetBlock(e.Chain.GetHeaderHash(e.Chain.BlockHeight()))
	require.NoError(t, err)
	return b
}

// NativeHash returns a native contract hash by the name.
func (e *Executor) NativeHash(t testing.TB, name string) util.Uint160 {
	h, err := e.Chain.GetNativeContractScriptHash(name)
	require.NoError(t, err)
	return h
}

// ContractHash returns a contract hash by the ID.
func (e *Executor) ContractHash(t testing.TB, id int32) util.Uint160 {
	h, err := e.Chain.GetContractScriptHash(id)
	require.NoError(t, err)
	return h
}

// NativeID returns a native contract ID by the name.
func (e *Executor) NativeID(t testing.TB, name string) int32 {
	h := e.NativeHash(t, name)
	cs := e.Chain.GetContractState(h)
	require.NotNil(t, cs)
	return cs.ID
}

// NewUnsignedTx creates a new unsigned transaction which invokes the method of the contract with the hash.
func (e *Executor) NewUnsignedTx(t testing.TB, hash util.Uint160, method string, args ...any) *transaction.Transaction {
	script, err := smartcontract.CreateCallScript(hash, method, args...)
	require.NoError(t, err)

	tx := transaction.New(script, 0)
	tx.Nonce = Nonce()
	tx.ValidUntilBlock = e.Chain.BlockHeight() + 1
	return tx
}

// NewTx creates a new transaction which invokes the contract method.
// The transaction is signed by the signers.
func (e *Executor) NewTx(t testing.TB, signers []Signer,
	hash util.Uint160, method string, args ...any) *transaction.Transaction {
	tx := e.NewUnsignedTx(t, hash, method, args...)
	return e.SignTx(t, tx, -1, signers...)
}

// SignTx signs a transaction using the provided signers.
func (e *Executor) SignTx(t testing.TB, tx *transaction.Transaction, sysFee int64, signers ...Signer) *transaction.Transaction {
	for _, acc := range signers {
		tx.Signers = append(tx.Signers, transaction.Signer{
			Account: acc.ScriptHash(),
			Scopes:  transaction.Global,
		})
	}
	AddNetworkFee(t, e.Chain, tx, signers...)
	AddSystemFee(e.Chain, tx, sysFee)

	for _, acc := range signers {
		require.NoError(t, acc.SignTx(e.Chain.GetConfig().Magic, tx))
	}
	return tx
}

// NewAccount returns a new signer holding 100.0 GAS (or given amount is specified).
// This method advances the chain by one block with a transfer transaction.
func (e *Executor) NewAccount(t testing.TB, expectedGASBalance ...int64) Signer {
	acc, err := wallet.NewAccount()
	require.NoError(t, err)

	amount := int64(100_0000_0000)
	if len(expectedGASBalance) != 0 {
		amount = expectedGASBalance[0]
	}
	tx := e.NewTx(t, []Signer{e.Validator},
		e.NativeHash(t, nativenames.Gas), "transfer",
		e.Validator.ScriptHash(), acc.Contract.ScriptHash(), amount, nil)
	e.AddNewBlock(t, tx)
	e.CheckHalt(t, tx.Hash())
	return NewSingleSigner(acc)
}

// DeployContract compiles and deploys a contract to the bc. It also checks that
// the precalculated contract hash matches the actual one.
// data is an optional argument to `_deploy`.
// It returns the hash of the deploy transaction.
func (e *Executor) DeployContract(t testing.TB, c *Contract, data any) util.Uint256 {
	return e.DeployContractBy(t, e.Validator, c, data)
}

// DeployContractBy compiles and deploys a contract to the bc using the provided signer.
// It also checks that the precalculated contract hash matches the actual one.
// data is an optional argument to `_deploy`.
// It returns the hash of the deploy transaction.
func (e *Executor) DeployContractBy(t testing.TB, signer Signer, c *Contract, data any) util.Uint256 {
	e.trackCoverage(t, c)

	tx := NewDeployTxBy(t, e.Chain, signer, c, data)
	e.AddNewBlock(t, tx)
	e.CheckHalt(t, tx.Hash())

	// Check that the precalculated hash matches the real one.
	e.CheckTxNotificationEvent(t, tx.Hash(), -1, state.NotificationEvent{
		ScriptHash: e.NativeHash(t, nativenames.Management),
		Name:       "Deploy",
		Item: stackitem.NewArray([]stackitem.Item{
			stackitem.NewByteArray(c.Hash.BytesBE()),
		}),
	})
	return tx.Hash()
}

// DeployContractCheckFAULT compiles and deploys a contract to the bc using the validator
// account. It checks that the deploy transaction FAULTed with the specified error.
func (e *Executor) DeployContractCheckFAULT(t testing.TB, c *Contract, data any, errMessage string) {
	e.trackCoverage(t, c)

	tx := e.NewDeployTx(t, e.Chain, c, data)
	e.AddNewBlock(t, tx)
	e.CheckFault(t, tx.Hash(), errMessage)
}

// This switches on coverage tracking for provided script if `go test` is running with coverage enabled.
func (e *Executor) trackCoverage(t testing.TB, c *Contract) {
	if e.collectCoverage {
		if _, ok := rawCoverage[c.Hash]; !ok {
			rawCoverage[c.Hash] = &scriptRawCoverage{debugInfo: c.DebugInfo}
		}
		t.Cleanup(func() {
			reportCoverage(t)
		})
	}
}

// InvokeScript adds a transaction with the specified script to the chain and
// returns its hash. It does no faults check.
func (e *Executor) InvokeScript(t testing.TB, script []byte, signers []Signer) util.Uint256 {
	tx := e.PrepareInvocation(t, script, signers)
	e.AddNewBlock(t, tx)
	return tx.Hash()
}

// PrepareInvocation creates a transaction with the specified script and signs it
// by the provided signer.
func (e *Executor) PrepareInvocation(t testing.TB, script []byte, signers []Signer, validUntilBlock ...uint32) *transaction.Transaction {
	tx := e.PrepareInvocationNoSign(t, script, validUntilBlock...)
	e.SignTx(t, tx, -1, signers...)
	return tx
}

func (e *Executor) PrepareInvocationNoSign(t testing.TB, script []byte, validUntilBlock ...uint32) *transaction.Transaction {
	tx := transaction.New(script, 0)
	tx.Nonce = Nonce()
	tx.ValidUntilBlock = e.Chain.BlockHeight() + 1
	if len(validUntilBlock) != 0 {
		tx.ValidUntilBlock = validUntilBlock[0]
	}
	return tx
}

// InvokeScriptCheckHALT adds a transaction with the specified script to the chain
// and checks if it's HALTed with the specified items on stack.
func (e *Executor) InvokeScriptCheckHALT(t testing.TB, script []byte, signers []Signer, stack ...stackitem.Item) {
	hash := e.InvokeScript(t, script, signers)
	e.CheckHalt(t, hash, stack...)
}

// InvokeScriptCheckFAULT adds a transaction with the specified script to the
// chain and checks if it's FAULTed with the specified error.
func (e *Executor) InvokeScriptCheckFAULT(t testing.TB, script []byte, signers []Signer, errMessage string) util.Uint256 {
	hash := e.InvokeScript(t, script, signers)
	e.CheckFault(t, hash, errMessage)
	return hash
}

// CheckHalt checks that the transaction is persisted with HALT state.
func (e *Executor) CheckHalt(t testing.TB, h util.Uint256, stack ...stackitem.Item) *state.AppExecResult {
	aer, err := e.Chain.GetAppExecResults(h, trigger.Application)
	require.NoError(t, err)
	require.Equal(t, vmstate.Halt, aer[0].VMState, aer[0].FaultException)
	if len(stack) != 0 {
		require.Equal(t, stack, aer[0].Stack)
	}
	return &aer[0]
}

// CheckFault checks that the transaction is persisted with FAULT state.
// The raised exception is also checked to contain the s as a substring.
func (e *Executor) CheckFault(t testing.TB, h util.Uint256, s string) {
	aer, err := e.Chain.GetAppExecResults(h, trigger.Application)
	require.NoError(t, err)
	require.Equal(t, vmstate.Fault, aer[0].VMState)
	require.True(t, strings.Contains(aer[0].FaultException, s),
		"expected: %s, got: %s", s, aer[0].FaultException)
}

// CheckTxNotificationEvent checks that the specified event was emitted at the specified position
// during transaction script execution. Negative index corresponds to backwards enumeration.
func (e *Executor) CheckTxNotificationEvent(t testing.TB, h util.Uint256, index int, expected state.NotificationEvent) {
	aer, err := e.Chain.GetAppExecResults(h, trigger.Application)
	require.NoError(t, err)
	l := len(aer[0].Events)
	if index < 0 {
		index = l + index
	}
	require.True(t, 0 <= index && index < l, fmt.Errorf("notification index is out of range: want %d, len is %d", index, l))
	require.Equal(t, expected, aer[0].Events[index])
}

// CheckGASBalance ensures that the provided account owns the specified amount of GAS.
func (e *Executor) CheckGASBalance(t testing.TB, acc util.Uint160, expected *big.Int) {
	actual := e.Chain.GetUtilityTokenBalance(acc)
	require.Equal(t, expected, actual, fmt.Errorf("invalid GAS balance: expected %s, got %s", expected.String(), actual.String()))
}

// EnsureGASBalance ensures that the provided account owns the amount of GAS that satisfies the provided condition.
func (e *Executor) EnsureGASBalance(t testing.TB, acc util.Uint160, isOk func(balance *big.Int) bool) {
	actual := e.Chain.GetUtilityTokenBalance(acc)
	require.True(t, isOk(actual), fmt.Errorf("invalid GAS balance: got %s, condition is not satisfied", actual.String()))
}

// NewDeployTx returns a new deployment tx for the contract signed by the committee.
func (e *Executor) NewDeployTx(t testing.TB, bc *core.Blockchain, c *Contract, data any) *transaction.Transaction {
	return NewDeployTxBy(t, bc, e.Validator, c, data)
}

// NewDeployTxBy returns a new deployment tx for the contract signed by the specified signer.
func NewDeployTxBy(t testing.TB, bc *core.Blockchain, signer Signer, c *Contract, data any) *transaction.Transaction {
	rawManifest, err := json.Marshal(c.Manifest)
	require.NoError(t, err)

	neb, err := c.NEF.Bytes()
	require.NoError(t, err)

	script, err := smartcontract.CreateCallScript(bc.ManagementContractHash(), "deploy", neb, rawManifest, data)
	require.NoError(t, err)

	tx := transaction.New(script, 100*native.GASFactor)
	tx.Nonce = Nonce()
	tx.ValidUntilBlock = bc.BlockHeight() + 1
	tx.Signers = []transaction.Signer{{
		Account: signer.ScriptHash(),
		Scopes:  transaction.Global,
	}}
	AddNetworkFee(t, bc, tx, signer)
	require.NoError(t, signer.SignTx(netmode.UnitTestNet, tx))
	return tx
}

// AddSystemFee adds system fee to the transaction. If negative value specified,
// then system fee is defined by test invocation.
func AddSystemFee(bc *core.Blockchain, tx *transaction.Transaction, sysFee int64) {
	if sysFee >= 0 {
		tx.SystemFee = sysFee
		return
	}
	v, _ := TestInvoke(bc, tx) // ignore error to support failing transactions
	tx.SystemFee = v.GasConsumed()
}

// AddNetworkFee adds network fee to the transaction.
func AddNetworkFee(t testing.TB, bc *core.Blockchain, tx *transaction.Transaction, signers ...Signer) {
	baseFee := bc.GetBaseExecFee()
	size := io.GetVarSize(tx)
	for _, sgr := range signers {
		csgr, ok := sgr.(SingleSigner)
		if ok && csgr.Account().Contract.InvocationBuilder != nil {
			sc, err := csgr.Account().Contract.InvocationBuilder(tx)
			require.NoError(t, err)

			txCopy := *tx
			ic, err := bc.GetTestVM(trigger.Verification, &txCopy, nil)
			require.NoError(t, err)

			ic.UseSigners(tx.Signers)
			ic.VM.GasLimit = bc.GetMaxVerificationGAS()

			require.NoError(t, bc.InitVerificationContext(ic, csgr.ScriptHash(), &transaction.Witness{InvocationScript: sc, VerificationScript: csgr.Script()}))
			require.NoError(t, ic.VM.Run())

			tx.NetworkFee += ic.VM.GasConsumed()
			size += io.GetVarSize(sc) + io.GetVarSize(csgr.Script())
		} else {
			netFee, sizeDelta := fee.Calculate(baseFee, sgr.Script())
			tx.NetworkFee += netFee
			size += sizeDelta
		}
	}
	tx.NetworkFee += int64(size)*bc.FeePerByte() + bc.CalculateAttributesFee(tx)
}

// NewUnsignedBlock creates a new unsigned block from txs.
func (e *Executor) NewUnsignedBlock(t testing.TB, txs ...*transaction.Transaction) *block.Block {
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

// AddNewBlock creates a new block from the provided transactions and adds it on the bc.
func (e *Executor) AddNewBlock(t testing.TB, txs ...*transaction.Transaction) *block.Block {
	b := e.NewUnsignedBlock(t, txs...)
	e.SignBlock(b)
	require.NoError(t, e.Chain.AddBlock(b))
	return b
}

// GenerateNewBlocks adds the specified number of empty blocks to the chain.
func (e *Executor) GenerateNewBlocks(t testing.TB, count int) []*block.Block {
	blocks := make([]*block.Block, count)
	for i := 0; i < count; i++ {
		blocks[i] = e.AddNewBlock(t)
	}
	return blocks
}

// SignBlock add validators signature to b.
func (e *Executor) SignBlock(b *block.Block) *block.Block {
	invoc := e.Validator.SignHashable(uint32(e.Chain.GetConfig().Magic), b)
	b.Script.InvocationScript = invoc
	return b
}

// AddBlockCheckHalt is a convenient wrapper over AddBlock and CheckHalt.
func (e *Executor) AddBlockCheckHalt(t testing.TB, txs ...*transaction.Transaction) *block.Block {
	b := e.AddNewBlock(t, txs...)
	for _, tx := range txs {
		e.CheckHalt(t, tx.Hash())
	}
	return b
}

// TestInvoke creates a test VM with a dummy block and executes a transaction in it.
func TestInvoke(bc *core.Blockchain, tx *transaction.Transaction) (*vm.VM, error) {
	lastBlock, err := bc.GetBlock(bc.GetHeaderHash(bc.BlockHeight()))
	if err != nil {
		return nil, err
	}
	b := &block.Block{
		Header: block.Header{
			Index:     bc.BlockHeight() + 1,
			Timestamp: lastBlock.Timestamp + 1,
		},
	}

	// `GetTestVM` as well as `Run` can use a transaction hash which will set a cached value.
	// This is unwanted behavior, so we explicitly copy the transaction to perform execution.
	ttx := *tx
	ic, _ := bc.GetTestVM(trigger.Application, &ttx, b)

	if isCoverageEnabled() {
		ic.VM.SetOnExecHook(coverageHook)
	}

	defer ic.Finalize()

	ic.VM.LoadWithFlags(tx.Script, callflag.All)
	err = ic.VM.Run()
	return ic.VM, err
}

// GetTransaction returns a transaction and its height by the specified hash.
func (e *Executor) GetTransaction(t testing.TB, h util.Uint256) (*transaction.Transaction, uint32) {
	tx, height, err := e.Chain.GetTransaction(h)
	require.NoError(t, err)
	return tx, height
}

// GetBlockByIndex returns a block by the specified index.
func (e *Executor) GetBlockByIndex(t testing.TB, idx uint32) *block.Block {
	h := e.Chain.GetHeaderHash(idx)
	require.NotEmpty(t, h)
	b, err := e.Chain.GetBlock(h)
	require.NoError(t, err)
	return b
}

// GetTxExecResult returns application execution results for the specified transaction.
func (e *Executor) GetTxExecResult(t testing.TB, h util.Uint256) *state.AppExecResult {
	aer, err := e.Chain.GetAppExecResults(h, trigger.Application)
	require.NoError(t, err)
	require.Equal(t, 1, len(aer))
	return &aer[0]
}

// EnableCoverage enables coverage collection for this executor, but only when `go test` is running with coverage enabled.
func (e *Executor) EnableCoverage() {
	e.collectCoverage = isCoverageEnabled()
}

// DisableCoverage disables coverage collection for this executor until enabled explicitly through EnableCoverage.
func (e *Executor) DisableCoverage() {
	e.collectCoverage = false
}
