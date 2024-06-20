package actor

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/nspcc-dev/neo-go/pkg/vm/vmstate"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/stretchr/testify/require"
)

type RPCClient struct {
	err     error
	invRes  *result.Invoke
	netFee  int64
	bCount  atomic.Uint32
	version *result.Version
	hash    util.Uint256
	appLog  *result.ApplicationLog
	context context.Context
}

func (r *RPCClient) InvokeContractVerify(contract util.Uint160, params []smartcontract.Parameter, signers []transaction.Signer, witnesses ...transaction.Witness) (*result.Invoke, error) {
	return r.invRes, r.err
}
func (r *RPCClient) InvokeFunction(contract util.Uint160, operation string, params []smartcontract.Parameter, signers []transaction.Signer) (*result.Invoke, error) {
	return r.invRes, r.err
}
func (r *RPCClient) InvokeScript(script []byte, signers []transaction.Signer) (*result.Invoke, error) {
	return r.invRes, r.err
}
func (r *RPCClient) CalculateNetworkFee(tx *transaction.Transaction) (int64, error) {
	return r.netFee, r.err
}
func (r *RPCClient) GetBlockCount() (uint32, error) {
	return r.bCount.Load(), r.err
}
func (r *RPCClient) GetVersion() (*result.Version, error) {
	verCopy := *r.version
	return &verCopy, r.err
}
func (r *RPCClient) SendRawTransaction(tx *transaction.Transaction) (util.Uint256, error) {
	return r.hash, r.err
}
func (r *RPCClient) TerminateSession(sessionID uuid.UUID) (bool, error) {
	return false, nil // Just a stub, unused by actor.
}
func (r *RPCClient) TraverseIterator(sessionID, iteratorID uuid.UUID, maxItemsCount int) ([]stackitem.Item, error) {
	return nil, nil // Just a stub, unused by actor.
}
func (r *RPCClient) Context() context.Context {
	if r.context == nil {
		return context.Background()
	}
	return r.context
}

func (r *RPCClient) GetApplicationLog(hash util.Uint256, trig *trigger.Type) (*result.ApplicationLog, error) {
	if r.appLog != nil {
		return r.appLog, nil
	}
	return nil, errors.New("not found")
}
func testRPCAndAccount(t *testing.T) (*RPCClient, *wallet.Account) {
	client := &RPCClient{
		version: &result.Version{
			Protocol: result.Protocol{
				Network:              netmode.UnitTestNet,
				MillisecondsPerBlock: 1000,
				ValidatorsCount:      7,
			},
		},
	}
	acc, err := wallet.NewAccount()
	require.NoError(t, err)
	return client, acc
}

func TestNew(t *testing.T) {
	client, acc := testRPCAndAccount(t)

	// No signers.
	_, err := New(client, nil)
	require.Error(t, err)

	_, err = New(client, []SignerAccount{})
	require.Error(t, err)

	_, err = NewTuned(client, []SignerAccount{}, NewDefaultOptions())
	require.Error(t, err)

	// Good simple.
	a, err := NewSimple(client, acc)
	require.NoError(t, err)
	require.Equal(t, 1, len(a.signers))
	require.Equal(t, 1, len(a.txSigners))
	require.Equal(t, transaction.CalledByEntry, a.signers[0].Signer.Scopes)
	require.Equal(t, transaction.CalledByEntry, a.txSigners[0].Scopes)

	// Contractless account.
	badAcc, err := wallet.NewAccount()
	require.NoError(t, err)
	badAccHash := badAcc.Contract.ScriptHash()
	badAcc.Contract = nil

	signers := []SignerAccount{{
		Signer: transaction.Signer{
			Account: acc.Contract.ScriptHash(),
			Scopes:  transaction.None,
		},
		Account: acc,
	}, {
		Signer: transaction.Signer{
			Account: badAccHash,
			Scopes:  transaction.CalledByEntry,
		},
		Account: badAcc,
	}}

	_, err = New(client, signers)
	require.Error(t, err)

	// GetVersion returning error.
	client.err = errors.New("bad")
	_, err = NewSimple(client, acc)
	require.Error(t, err)
	client.err = nil

	// Account mismatch.
	acc2, err := wallet.NewAccount()
	require.NoError(t, err)
	signers = []SignerAccount{{
		Signer: transaction.Signer{
			Account: acc2.Contract.ScriptHash(),
			Scopes:  transaction.None,
		},
		Account: acc,
	}, {
		Signer: transaction.Signer{
			Account: acc2.Contract.ScriptHash(),
			Scopes:  transaction.CalledByEntry,
		},
		Account: acc2,
	}}
	_, err = New(client, signers)
	require.Error(t, err)

	// Good multiaccount.
	signers[0].Signer.Account = acc.Contract.ScriptHash()
	a, err = New(client, signers)
	require.NoError(t, err)
	require.Equal(t, 2, len(a.signers))
	require.Equal(t, 2, len(a.txSigners))

	// Good tuned
	opts := Options{
		Attributes: []transaction.Attribute{{Type: transaction.HighPriority}},
	}
	a, err = NewTuned(client, signers, opts)
	require.NoError(t, err)
	require.Equal(t, 1, len(a.opts.Attributes))
}

func TestSimpleWrappers(t *testing.T) {
	client, acc := testRPCAndAccount(t)
	origVer := *client.version

	a, err := NewSimple(client, acc)
	require.NoError(t, err)

	client.netFee = 42
	nf, err := a.CalculateNetworkFee(new(transaction.Transaction))
	require.NoError(t, err)
	require.Equal(t, int64(42), nf)

	client.bCount.Store(100500)
	bc, err := a.GetBlockCount()
	require.NoError(t, err)
	require.Equal(t, uint32(100500), bc)

	require.Equal(t, netmode.UnitTestNet, a.GetNetwork())
	client.version.Protocol.Network = netmode.TestNet
	require.Equal(t, netmode.UnitTestNet, a.GetNetwork())
	require.Equal(t, origVer, a.GetVersion())

	a, err = NewSimple(client, acc)
	require.NoError(t, err)
	require.Equal(t, netmode.TestNet, a.GetNetwork())
	require.Equal(t, *client.version, a.GetVersion())

	client.hash = util.Uint256{1, 2, 3}
	h, vub, err := a.Send(&transaction.Transaction{ValidUntilBlock: 123})
	require.NoError(t, err)
	require.Equal(t, client.hash, h)
	require.Equal(t, uint32(123), vub)
}

func TestSign(t *testing.T) {
	client, acc := testRPCAndAccount(t)
	acc2, err := wallet.NewAccount()
	require.NoError(t, err)

	a, err := New(client, []SignerAccount{{
		Signer: transaction.Signer{
			Account: acc.Contract.ScriptHash(),
			Scopes:  transaction.None,
		},
		Account: acc,
	}, {
		Signer: transaction.Signer{
			Account: acc2.Contract.ScriptHash(),
			Scopes:  transaction.CalledByEntry,
		},
		Account: &wallet.Account{ // Looks like acc2, but has no private key.
			Address:      acc2.Address,
			EncryptedWIF: acc2.EncryptedWIF,
			Contract:     acc2.Contract,
		},
	}})
	require.NoError(t, err)

	script := []byte{1, 2, 3}
	client.hash = util.Uint256{2, 5, 6}
	client.invRes = &result.Invoke{State: "HALT", GasConsumed: 3, Script: script}

	tx, err := a.MakeUnsignedRun(script, nil)
	require.NoError(t, err)
	require.Error(t, a.Sign(tx))
	_, _, err = a.SignAndSend(tx)
	require.Error(t, err)
}

func TestSenders(t *testing.T) {
	client, acc := testRPCAndAccount(t)
	a, err := NewSimple(client, acc)
	require.NoError(t, err)
	script := []byte{1, 2, 3}

	// Bad.
	client.invRes = &result.Invoke{State: "FAULT", GasConsumed: 3, Script: script}
	_, _, err = a.SendCall(util.Uint160{1}, "method", 42)
	require.Error(t, err)
	_, _, err = a.SendTunedCall(util.Uint160{1}, "method", nil, nil, 42)
	require.Error(t, err)
	_, _, err = a.SendRun(script)
	require.Error(t, err)
	_, _, err = a.SendTunedRun(script, nil, nil)
	require.Error(t, err)
	_, _, err = a.SendUncheckedRun(script, 1, nil, func(t *transaction.Transaction) error {
		return errors.New("bad")
	})
	require.Error(t, err)

	// Good.
	client.hash = util.Uint256{2, 5, 6}
	client.invRes = &result.Invoke{State: "HALT", GasConsumed: 3, Script: script}
	h, vub, err := a.SendCall(util.Uint160{1}, "method", 42)
	require.NoError(t, err)
	require.Equal(t, client.hash, h)
	require.Equal(t, uint32(8), vub)

	h, vub, err = a.SendTunedCall(util.Uint160{1}, "method", nil, nil, 42)
	require.NoError(t, err)
	require.Equal(t, client.hash, h)
	require.Equal(t, uint32(8), vub)

	h, vub, err = a.SendRun(script)
	require.NoError(t, err)
	require.Equal(t, client.hash, h)
	require.Equal(t, uint32(8), vub)

	h, vub, err = a.SendTunedRun(script, nil, nil)
	require.NoError(t, err)
	require.Equal(t, client.hash, h)
	require.Equal(t, uint32(8), vub)

	h, vub, err = a.SendUncheckedRun(script, 1, nil, nil)
	require.NoError(t, err)
	require.Equal(t, client.hash, h)
	require.Equal(t, uint32(8), vub)
}

func TestSender(t *testing.T) {
	client, acc := testRPCAndAccount(t)
	a, err := NewSimple(client, acc)
	require.NoError(t, err)
	require.Equal(t, acc.ScriptHash(), a.Sender())
}

func TestWaitSuccess(t *testing.T) {
	client, acc := testRPCAndAccount(t)
	a, err := NewSimple(client, acc)
	require.NoError(t, err)

	someErr := errors.New("someErr")
	_, err = a.WaitSuccess(util.Uint256{}, 0, someErr)
	require.ErrorIs(t, err, someErr)

	cont := util.Uint256{1, 2, 3}
	ex := state.Execution{
		Trigger:     trigger.Application,
		VMState:     vmstate.Halt,
		GasConsumed: 123,
		Stack:       []stackitem.Item{stackitem.Null{}},
	}
	applog := &result.ApplicationLog{
		Container:     cont,
		IsTransaction: true,
		Executions:    []state.Execution{ex},
	}
	client.appLog = applog
	client.appLog.Executions[0].VMState = vmstate.Fault
	_, err = a.WaitSuccess(util.Uint256{}, 0, nil)
	require.ErrorIs(t, err, ErrExecFailed)

	client.appLog.Executions[0].VMState = vmstate.Halt
	res, err := a.WaitSuccess(util.Uint256{}, 0, nil)
	require.NoError(t, err)
	require.Equal(t, &state.AppExecResult{
		Container: cont,
		Execution: ex,
	}, res)
}
