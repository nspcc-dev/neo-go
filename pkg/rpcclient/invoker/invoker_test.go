package invoker

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

type rpcInv struct {
	resInv *result.Invoke
	resTrm bool
	resItm []stackitem.Item
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
func (r *rpcInv) InvokeContractVerifyAtHeight(height uint32, contract util.Uint160, params []smartcontract.Parameter, signers []transaction.Signer, witnesses ...transaction.Witness) (*result.Invoke, error) {
	return r.resInv, r.err
}
func (r *rpcInv) InvokeContractVerifyWithState(stateroot util.Uint256, contract util.Uint160, params []smartcontract.Parameter, signers []transaction.Signer, witnesses ...transaction.Witness) (*result.Invoke, error) {
	return r.resInv, r.err
}
func (r *rpcInv) InvokeFunctionAtHeight(height uint32, contract util.Uint160, operation string, params []smartcontract.Parameter, signers []transaction.Signer) (*result.Invoke, error) {
	return r.resInv, r.err
}
func (r *rpcInv) InvokeFunctionWithState(stateroot util.Uint256, contract util.Uint160, operation string, params []smartcontract.Parameter, signers []transaction.Signer) (*result.Invoke, error) {
	return r.resInv, r.err
}
func (r *rpcInv) InvokeScriptAtHeight(height uint32, script []byte, signers []transaction.Signer) (*result.Invoke, error) {
	return r.resInv, r.err
}
func (r *rpcInv) InvokeScriptWithState(stateroot util.Uint256, script []byte, signers []transaction.Signer) (*result.Invoke, error) {
	return r.resInv, r.err
}
func (r *rpcInv) TerminateSession(sessionID uuid.UUID) (bool, error) {
	return r.resTrm, r.err
}
func (r *rpcInv) TraverseIterator(sessionID, iteratorID uuid.UUID, maxItemsCount int) ([]stackitem.Item, error) {
	if r.err != nil {
		return nil, r.err
	}
	maxItemsCount = min(maxItemsCount, len(r.resItm))
	items := r.resItm[:maxItemsCount]
	r.resItm = r.resItm[maxItemsCount:]
	return items, nil
}

func TestInvoker(t *testing.T) {
	resExp := &result.Invoke{State: "HALT"}
	ri := &rpcInv{resExp, true, nil, nil}

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

		_, err = inv.Verify(util.Uint160{}, nil, make(chan struct{}))
		require.Error(t, err)

		_, err = inv.Call(util.Uint160{}, "method", make(chan struct{}))
		require.Error(t, err)

		res, err = inv.CallAndExpandIterator(util.Uint160{}, "method", 10, 42)
		require.NoError(t, err)
		require.Equal(t, resExp, res)

		_, err = inv.CallAndExpandIterator(util.Uint160{}, "method", 10, make(chan struct{}))
		require.Error(t, err)
	}
	t.Run("standard", func(t *testing.T) {
		testInv(t, New(ri, nil))
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
	t.Run("terminate session", func(t *testing.T) {
		for _, inv := range []*Invoker{New(ri, nil), NewHistoricWithState(util.Uint256{}, ri, nil)} {
			ri.err = errors.New("")
			require.Error(t, inv.TerminateSession(uuid.UUID{}))
			ri.err = nil
			ri.resTrm = false
			require.Error(t, inv.TerminateSession(uuid.UUID{}))
			ri.resTrm = true
			require.NoError(t, inv.TerminateSession(uuid.UUID{}))
		}
	})
	t.Run("traverse iterator", func(t *testing.T) {
		for _, inv := range []*Invoker{New(ri, nil), NewHistoricWithState(util.Uint256{}, ri, nil)} {
			res, err := inv.TraverseIterator(uuid.UUID{}, &result.Iterator{
				Values: []stackitem.Item{stackitem.Make(42)},
			}, 0)
			require.NoError(t, err)
			require.Equal(t, []stackitem.Item{stackitem.Make(42)}, res)

			res, err = inv.TraverseIterator(uuid.UUID{}, &result.Iterator{
				Values: []stackitem.Item{stackitem.Make(42)},
			}, 1)
			require.NoError(t, err)
			require.Equal(t, []stackitem.Item{stackitem.Make(42)}, res)

			res, err = inv.TraverseIterator(uuid.UUID{}, &result.Iterator{
				Values: []stackitem.Item{stackitem.Make(42)},
			}, 2)
			require.NoError(t, err)
			require.Equal(t, []stackitem.Item{stackitem.Make(42)}, res)

			ri.err = errors.New("")
			_, err = inv.TraverseIterator(uuid.UUID{}, &result.Iterator{
				ID: &uuid.UUID{},
			}, 2)
			require.Error(t, err)

			ri.err = nil
			ri.resItm = []stackitem.Item{stackitem.Make(42)}
			res, err = inv.TraverseIterator(uuid.UUID{}, &result.Iterator{
				ID: &uuid.UUID{},
			}, 2)
			require.NoError(t, err)
			require.Equal(t, []stackitem.Item{stackitem.Make(42)}, res)
		}
	})
	t.Run("traverse iterator with session expansion", func(t *testing.T) {
		mockClient := &rpcInv{
			resItm: []stackitem.Item{
				stackitem.Make(1),
				stackitem.Make(2),
				stackitem.Make(3),
			},
			err: nil,
		}
		inv := New(mockClient, nil)

		sessionID := uuid.New()
		iteratorID := uuid.New()
		iter := &result.Iterator{
			ID:     &iteratorID,
			Values: []stackitem.Item{stackitem.Make(10), stackitem.Make(20)},
		}
		res, err := inv.TraverseIterator(sessionID, iter, 2)
		require.NoError(t, err)
		require.Equal(t, []stackitem.Item{stackitem.Make(10), stackitem.Make(20)}, res)

		res, err = inv.TraverseIterator(sessionID, iter, 2)
		require.NoError(t, err)
		require.Equal(t, []stackitem.Item{stackitem.Make(1), stackitem.Make(2)}, res)

		res, err = inv.TraverseIterator(sessionID, iter, 2)
		require.NoError(t, err)
		require.Equal(t, []stackitem.Item{stackitem.Make(3)}, res)

		mockClient.resItm = nil
		res, err = inv.TraverseIterator(sessionID, iter, 2)
		require.NoError(t, err)
		require.Empty(t, res)
	})
}

func TestInvokerSigners(t *testing.T) {
	resExp := &result.Invoke{State: "HALT"}
	ri := &rpcInv{resExp, true, nil, nil}
	inv := New(ri, nil)

	require.Nil(t, inv.Signers())

	s := []transaction.Signer{}
	inv = New(ri, s)
	require.Equal(t, s, inv.Signers())

	s = append(s, transaction.Signer{Account: util.Uint160{1, 2, 3}, Scopes: transaction.CalledByEntry})
	inv = New(ri, s)
	require.Equal(t, s, inv.Signers())
}
