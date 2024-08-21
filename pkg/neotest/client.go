package neotest

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/nspcc-dev/neo-go/pkg/vm/vmstate"
	"github.com/stretchr/testify/require"
)

// ContractInvoker is a client for a specific contract.
type ContractInvoker struct {
	*Executor
	Hash    util.Uint160
	Signers []Signer
}

// NewInvoker creates a new ContractInvoker for the contract with hash h and the specified signers.
func (e *Executor) NewInvoker(h util.Uint160, signers ...Signer) *ContractInvoker {
	return &ContractInvoker{
		Executor: e,
		Hash:     h,
		Signers:  signers,
	}
}

// CommitteeInvoker creates a new ContractInvoker for the contract with hash h and a committee multisignature signer.
func (e *Executor) CommitteeInvoker(h util.Uint160) *ContractInvoker {
	return &ContractInvoker{
		Executor: e,
		Hash:     h,
		Signers:  []Signer{e.Committee},
	}
}

// ValidatorInvoker creates a new ContractInvoker for the contract with hash h and a validators multisignature signer.
func (e *Executor) ValidatorInvoker(h util.Uint160) *ContractInvoker {
	return &ContractInvoker{
		Executor: e,
		Hash:     h,
		Signers:  []Signer{e.Validator},
	}
}

// TestInvokeScript creates test VM and invokes the script with the args and signers.
func (c *ContractInvoker) TestInvokeScript(t testing.TB, script []byte, signers []Signer, validUntilBlock ...uint32) (*vm.Stack, error) {
	tx := c.PrepareInvocationNoSign(t, script, validUntilBlock...)
	for _, acc := range signers {
		tx.Signers = append(tx.Signers, transaction.Signer{
			Account: acc.ScriptHash(),
			Scopes:  transaction.Global,
		})
	}
	b := c.NewUnsignedBlock(t, tx)
	ic, err := c.Chain.GetTestVM(trigger.Application, tx, b)
	if err != nil {
		return nil, err
	}
	t.Cleanup(ic.Finalize)

	if c.collectCoverage {
		ic.VM.SetOnExecHook(coverageHook)
	}

	ic.VM.LoadWithFlags(tx.Script, callflag.All)
	err = ic.VM.Run()
	return ic.VM.Estack(), err
}

// TestInvoke creates test VM and invokes the method with the args.
func (c *ContractInvoker) TestInvoke(t testing.TB, method string, args ...any) (*vm.Stack, error) {
	tx := c.PrepareInvokeNoSign(t, method, args...)
	b := c.NewUnsignedBlock(t, tx)
	ic, err := c.Chain.GetTestVM(trigger.Application, tx, b)
	if err != nil {
		return nil, err
	}
	t.Cleanup(ic.Finalize)

	if c.collectCoverage {
		ic.VM.SetOnExecHook(coverageHook)
	}

	ic.VM.LoadWithFlags(tx.Script, callflag.All)
	err = ic.VM.Run()
	return ic.VM.Estack(), err
}

// WithSigners creates a new client with the provided signer.
func (c *ContractInvoker) WithSigners(signers ...Signer) *ContractInvoker {
	newC := *c
	newC.Signers = signers
	return &newC
}

// PrepareInvoke creates a new invocation transaction.
func (c *ContractInvoker) PrepareInvoke(t testing.TB, method string, args ...any) *transaction.Transaction {
	return c.Executor.NewTx(t, c.Signers, c.Hash, method, args...)
}

// PrepareInvokeNoSign creates a new unsigned invocation transaction.
func (c *ContractInvoker) PrepareInvokeNoSign(t testing.TB, method string, args ...any) *transaction.Transaction {
	return c.Executor.NewUnsignedTx(t, c.Hash, method, args...)
}

// Invoke invokes the method with the args, persists the transaction and checks the result.
// Returns transaction hash.
func (c *ContractInvoker) Invoke(t testing.TB, result any, method string, args ...any) util.Uint256 {
	tx := c.PrepareInvoke(t, method, args...)
	c.AddNewBlock(t, tx)
	c.CheckHalt(t, tx.Hash(), stackitem.Make(result))
	return tx.Hash()
}

// InvokeAndCheck invokes the method with the args, persists the transaction and checks the result
// using the provided function. It returns the transaction hash.
func (c *ContractInvoker) InvokeAndCheck(t testing.TB, checkResult func(t testing.TB, stack []stackitem.Item), method string, args ...any) util.Uint256 {
	tx := c.PrepareInvoke(t, method, args...)
	c.AddNewBlock(t, tx)
	aer, err := c.Chain.GetAppExecResults(tx.Hash(), trigger.Application)
	require.NoError(t, err)
	require.Equal(t, vmstate.Halt, aer[0].VMState, aer[0].FaultException)
	if checkResult != nil {
		checkResult(t, aer[0].Stack)
	}
	return tx.Hash()
}

// InvokeWithFeeFail is like InvokeFail but sets the custom system fee for the transaction.
func (c *ContractInvoker) InvokeWithFeeFail(t testing.TB, message string, sysFee int64, method string, args ...any) util.Uint256 {
	tx := c.PrepareInvokeNoSign(t, method, args...)
	c.Executor.SignTx(t, tx, sysFee, c.Signers...)
	c.AddNewBlock(t, tx)
	c.CheckFault(t, tx.Hash(), message)
	return tx.Hash()
}

// InvokeFail invokes the method with the args, persists the transaction and checks the error message.
// It returns the transaction hash.
func (c *ContractInvoker) InvokeFail(t testing.TB, message string, method string, args ...any) util.Uint256 {
	tx := c.PrepareInvoke(t, method, args...)
	c.AddNewBlock(t, tx)
	c.CheckFault(t, tx.Hash(), message)
	return tx.Hash()
}
