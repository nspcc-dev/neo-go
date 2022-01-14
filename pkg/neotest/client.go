package neotest

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

// ContractInvoker is a client for specific contract.
type ContractInvoker struct {
	*Executor
	Hash    util.Uint160
	Signers []Signer
}

// CommitteeInvoker creates new ContractInvoker for contract with hash h and committee multisignature signer.
func (e *Executor) CommitteeInvoker(h util.Uint160) *ContractInvoker {
	return &ContractInvoker{
		Executor: e,
		Hash:     h,
		Signers:  []Signer{e.Committee},
	}
}

// ValidatorInvoker creates new ContractInvoker for contract with hash h and validators multisignature signer.
func (e *Executor) ValidatorInvoker(h util.Uint160) *ContractInvoker {
	return &ContractInvoker{
		Executor: e,
		Hash:     h,
		Signers:  []Signer{e.Validator},
	}
}

// TestInvoke creates test VM and invokes method with args.
func (c *ContractInvoker) TestInvoke(t *testing.T, method string, args ...interface{}) (*vm.Stack, error) {
	tx := c.PrepareInvokeNoSign(t, method, args...)
	b := c.NewUnsignedBlock(t, tx)
	ic := c.Chain.GetTestVM(trigger.Application, tx, b)
	t.Cleanup(ic.Finalize)

	ic.VM.LoadWithFlags(tx.Script, callflag.All)
	err := ic.VM.Run()
	return ic.VM.Estack(), err
}

// WithSigners creates new client with the provided signer.
func (c *ContractInvoker) WithSigners(signers ...Signer) *ContractInvoker {
	newC := *c
	newC.Signers = signers
	return &newC
}

// PrepareInvoke creates new invocation transaction.
func (c *ContractInvoker) PrepareInvoke(t *testing.T, method string, args ...interface{}) *transaction.Transaction {
	return c.Executor.NewTx(t, c.Signers, c.Hash, method, args...)
}

// PrepareInvokeNoSign creates new unsigned invocation transaction.
func (c *ContractInvoker) PrepareInvokeNoSign(t *testing.T, method string, args ...interface{}) *transaction.Transaction {
	return c.Executor.NewUnsignedTx(t, c.Hash, method, args...)
}

// Invoke invokes method with args, persists transaction and checks the result.
// Returns transaction hash.
func (c *ContractInvoker) Invoke(t *testing.T, result interface{}, method string, args ...interface{}) util.Uint256 {
	tx := c.PrepareInvoke(t, method, args...)
	c.AddNewBlock(t, tx)
	c.CheckHalt(t, tx.Hash(), stackitem.Make(result))
	return tx.Hash()
}

// InvokeAndCheck invokes method with args, persists transaction and checks the result
// using provided function. Returns transaction hash.
func (c *ContractInvoker) InvokeAndCheck(t *testing.T, checkResult func(t *testing.T, stack []stackitem.Item), method string, args ...interface{}) util.Uint256 {
	tx := c.PrepareInvoke(t, method, args...)
	c.AddNewBlock(t, tx)
	aer, err := c.Chain.GetAppExecResults(tx.Hash(), trigger.Application)
	require.NoError(t, err)
	require.Equal(t, vm.HaltState, aer[0].VMState, aer[0].FaultException)
	if checkResult != nil {
		checkResult(t, aer[0].Stack)
	}
	return tx.Hash()
}

// InvokeWithFeeFail is like InvokeFail but sets custom system fee for the transaction.
func (c *ContractInvoker) InvokeWithFeeFail(t *testing.T, message string, sysFee int64, method string, args ...interface{}) util.Uint256 {
	tx := c.PrepareInvokeNoSign(t, method, args...)
	c.Executor.SignTx(t, tx, sysFee, c.Signers...)
	c.AddNewBlock(t, tx)
	c.CheckFault(t, tx.Hash(), message)
	return tx.Hash()
}

// InvokeFail invokes method with args, persists transaction and checks the error message.
// Returns transaction hash.
func (c *ContractInvoker) InvokeFail(t *testing.T, message string, method string, args ...interface{}) {
	tx := c.PrepareInvoke(t, method, args...)
	c.AddNewBlock(t, tx)
	c.CheckFault(t, tx.Hash(), message)
}
