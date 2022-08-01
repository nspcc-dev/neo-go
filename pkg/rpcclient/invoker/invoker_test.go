package invoker

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

type rpcInv struct {
	resInv *result.Invoke
	err    error
}

func (r *rpcInv) InvokeContractVerify(contract util.Uint160, params []smartcontract.Parameter, signers []transaction.Signer, witnesses ...transaction.Witness) (*result.Invoke, error) {
	return r.resInv, r.err
}
func (r *rpcInv) InvokeFunction(contract util.Uint160, operation string, params []smartcontract.Parameter, signers []transaction.Signer) (*result.Invoke, error) {
	return r.resInv, r.err
}
func (r *rpcInv) InvokeScript(script []byte, signers []transaction.Signer) (*result.Invoke, error) {
	return r.resInv, r.err
}
func (r *rpcInv) InvokeContractVerifyAtBlock(blockHash util.Uint256, contract util.Uint160, params []smartcontract.Parameter, signers []transaction.Signer, witnesses ...transaction.Witness) (*result.Invoke, error) {
	return r.resInv, r.err
}
func (r *rpcInv) InvokeContractVerifyAtHeight(height uint32, contract util.Uint160, params []smartcontract.Parameter, signers []transaction.Signer, witnesses ...transaction.Witness) (*result.Invoke, error) {
	return r.resInv, r.err
}
func (r *rpcInv) InvokeContractVerifyWithState(stateroot util.Uint256, contract util.Uint160, params []smartcontract.Parameter, signers []transaction.Signer, witnesses ...transaction.Witness) (*result.Invoke, error) {
	return r.resInv, r.err
}
func (r *rpcInv) InvokeFunctionAtBlock(blockHash util.Uint256, contract util.Uint160, operation string, params []smartcontract.Parameter, signers []transaction.Signer) (*result.Invoke, error) {
	return r.resInv, r.err
}
func (r *rpcInv) InvokeFunctionAtHeight(height uint32, contract util.Uint160, operation string, params []smartcontract.Parameter, signers []transaction.Signer) (*result.Invoke, error) {
	return r.resInv, r.err
}
func (r *rpcInv) InvokeFunctionWithState(stateroot util.Uint256, contract util.Uint160, operation string, params []smartcontract.Parameter, signers []transaction.Signer) (*result.Invoke, error) {
	return r.resInv, r.err
}
func (r *rpcInv) InvokeScriptAtBlock(blockHash util.Uint256, script []byte, signers []transaction.Signer) (*result.Invoke, error) {
	return r.resInv, r.err
}
func (r *rpcInv) InvokeScriptAtHeight(height uint32, script []byte, signers []transaction.Signer) (*result.Invoke, error) {
	return r.resInv, r.err
}
func (r *rpcInv) InvokeScriptWithState(stateroot util.Uint256, script []byte, signers []transaction.Signer) (*result.Invoke, error) {
	return r.resInv, r.err
}

func TestInvoker(t *testing.T) {
	resExp := &result.Invoke{State: "HALT"}
	ri := &rpcInv{resExp, nil}

	testInv := func(t *testing.T, inv *Invoker) {
		res, err := inv.Call(util.Uint160{}, "method")
		require.NoError(t, err)
		require.Equal(t, resExp, res)

		res, err = inv.Verify(util.Uint160{}, nil)
		require.NoError(t, err)
		require.Equal(t, resExp, res)

		res, err = inv.Run([]byte{1})
		require.NoError(t, err)
		require.Equal(t, resExp, res)

		res, err = inv.Call(util.Uint160{}, "method")
		require.NoError(t, err)
		require.Equal(t, resExp, res)

		res, err = inv.Verify(util.Uint160{}, nil, "param")
		require.NoError(t, err)
		require.Equal(t, resExp, res)

		res, err = inv.Call(util.Uint160{}, "method", 42)
		require.NoError(t, err)
		require.Equal(t, resExp, res)

		_, err = inv.Verify(util.Uint160{}, nil, make(map[int]int))
		require.Error(t, err)

		_, err = inv.Call(util.Uint160{}, "method", make(map[int]int))
		require.Error(t, err)

		res, err = inv.CallAndExpandIterator(util.Uint160{}, "method", 10, 42)
		require.NoError(t, err)
		require.Equal(t, resExp, res)

		_, err = inv.CallAndExpandIterator(util.Uint160{}, "method", 10, make(map[int]int))
		require.Error(t, err)
	}
	t.Run("standard", func(t *testing.T) {
		testInv(t, New(ri, nil))
	})
	t.Run("historic, block", func(t *testing.T) {
		testInv(t, NewHistoricAtBlock(util.Uint256{}, ri, nil))
	})
	t.Run("historic, height", func(t *testing.T) {
		testInv(t, NewHistoricAtHeight(100500, ri, nil))
	})
	t.Run("historic, state", func(t *testing.T) {
		testInv(t, NewHistoricWithState(util.Uint256{}, ri, nil))
	})
	t.Run("broken historic", func(t *testing.T) {
		inv := New(&historicConverter{client: ri}, nil) // It's not possible to do this from outside.
		require.Panics(t, func() { _, _ = inv.Call(util.Uint160{}, "method") })
		require.Panics(t, func() { _, _ = inv.Verify(util.Uint160{}, nil, "param") })
		require.Panics(t, func() { _, _ = inv.Run([]byte{1}) })
	})
}
