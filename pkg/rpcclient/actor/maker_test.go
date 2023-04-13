package actor

import (
	"errors"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestCalculateValidUntilBlock(t *testing.T) {
	client, acc := testRPCAndAccount(t)
	a, err := NewSimple(client, acc)
	require.NoError(t, err)

	client.err = errors.New("error")
	_, err = a.CalculateValidUntilBlock()
	require.Error(t, err)

	client.err = nil
	client.bCount.Store(42)
	vub, err := a.CalculateValidUntilBlock()
	require.NoError(t, err)
	require.Equal(t, uint32(42+7+1), vub)

	client.version.Protocol.ValidatorsHistory = map[uint32]uint32{
		0:  7,
		40: 4,
		80: 10,
	}
	a, err = NewSimple(client, acc)
	require.NoError(t, err)

	vub, err = a.CalculateValidUntilBlock()
	require.NoError(t, err)
	require.Equal(t, uint32(42+4+1), vub)

	client.bCount.Store(101)
	vub, err = a.CalculateValidUntilBlock()
	require.NoError(t, err)
	require.Equal(t, uint32(101+10+1), vub)
}

func TestMakeUnsigned(t *testing.T) {
	client, acc := testRPCAndAccount(t)
	a, err := NewSimple(client, acc)
	require.NoError(t, err)

	// Bad parameters.
	script := []byte{1, 2, 3}
	_, err = a.MakeUnsignedUncheckedRun(script, -1, nil)
	require.Error(t, err)
	_, err = a.MakeUnsignedUncheckedRun([]byte{}, 1, nil)
	require.Error(t, err)
	_, err = a.MakeUnsignedUncheckedRun(nil, 1, nil)
	require.Error(t, err)

	// RPC error.
	client.err = errors.New("err")
	_, err = a.MakeUnsignedUncheckedRun(script, 1, nil)
	require.Error(t, err)

	// Good unchecked.
	client.netFee = 42
	client.bCount.Store(100500)
	client.err = nil
	tx, err := a.MakeUnsignedUncheckedRun(script, 1, nil)
	require.NoError(t, err)
	require.Equal(t, script, tx.Script)
	require.Equal(t, 1, len(tx.Signers))
	require.Equal(t, acc.Contract.ScriptHash(), tx.Signers[0].Account)
	require.Equal(t, 1, len(tx.Scripts))
	require.Equal(t, acc.Contract.Script, tx.Scripts[0].VerificationScript)
	require.Nil(t, tx.Scripts[0].InvocationScript)

	// Bad run.
	client.err = errors.New("")
	_, err = a.MakeUnsignedRun(script, nil)
	require.Error(t, err)

	// Faulted run.
	client.invRes = &result.Invoke{State: "FAULT", GasConsumed: 3, Script: script}
	client.err = nil
	_, err = a.MakeUnsignedRun(script, nil)
	require.Error(t, err)

	// Good run.
	client.invRes = &result.Invoke{State: "HALT", GasConsumed: 3, Script: script}
	_, err = a.MakeUnsignedRun(script, nil)
	require.NoError(t, err)

	// Tuned.
	opts := Options{
		Attributes: []transaction.Attribute{{Type: transaction.HighPriority}},
	}
	a, err = NewTuned(client, a.signers, opts)
	require.NoError(t, err)

	tx, err = a.MakeUnsignedRun(script, nil)
	require.NoError(t, err)
	require.True(t, tx.HasAttribute(transaction.HighPriority))
}

func TestMakeSigned(t *testing.T) {
	client, acc := testRPCAndAccount(t)
	a, err := NewSimple(client, acc)
	require.NoError(t, err)

	// Bad script.
	_, err = a.MakeUncheckedRun(nil, 0, nil, nil)
	require.Error(t, err)

	// Good, no hook.
	script := []byte{1, 2, 3}
	_, err = a.MakeUncheckedRun(script, 0, nil, nil)
	require.NoError(t, err)

	// Bad, can't sign because of a hook.
	_, err = a.MakeUncheckedRun(script, 0, nil, func(t *transaction.Transaction) error {
		t.Signers = append(t.Signers, transaction.Signer{})
		return nil
	})
	require.Error(t, err)

	// Bad, hook returns an error.
	_, err = a.MakeUncheckedRun(script, 0, nil, func(t *transaction.Transaction) error {
		return errors.New("")
	})
	require.Error(t, err)

	// Good with a hook.
	tx, err := a.MakeUncheckedRun(script, 0, nil, func(t *transaction.Transaction) error {
		t.ValidUntilBlock = 777
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, uint32(777), tx.ValidUntilBlock)

	// Tuned.
	opts := Options{
		Modifier: func(t *transaction.Transaction) error {
			t.ValidUntilBlock = 888
			return nil
		},
	}
	at, err := NewTuned(client, a.signers, opts)
	require.NoError(t, err)

	tx, err = at.MakeUncheckedRun(script, 0, nil, nil)
	require.NoError(t, err)
	require.Equal(t, uint32(888), tx.ValidUntilBlock)

	// Checked

	// Bad, invocation fails.
	client.err = errors.New("")
	_, err = a.MakeTunedRun(script, nil, func(r *result.Invoke, t *transaction.Transaction) error {
		return nil
	})
	require.Error(t, err)

	// Bad, hook returns an error.
	client.err = nil
	client.invRes = &result.Invoke{State: "HALT", GasConsumed: 3, Script: script}
	_, err = a.MakeTunedRun(script, nil, func(r *result.Invoke, t *transaction.Transaction) error {
		return errors.New("")
	})
	require.Error(t, err)

	// Good, no hook.
	_, err = a.MakeTunedRun(script, []transaction.Attribute{{Type: transaction.HighPriority}}, nil)
	require.NoError(t, err)
	_, err = a.MakeRun(script)
	require.NoError(t, err)

	// Bad, invocation returns FAULT.
	client.invRes = &result.Invoke{State: "FAULT", GasConsumed: 3, Script: script}
	_, err = a.MakeTunedRun(script, nil, nil)
	require.Error(t, err)

	// Good, invocation returns FAULT, but callback ignores it.
	_, err = a.MakeTunedRun(script, nil, func(r *result.Invoke, t *transaction.Transaction) error {
		return nil
	})
	require.NoError(t, err)

	// Good, via call and with a callback.
	_, err = a.MakeTunedCall(util.Uint160{}, "something", []transaction.Attribute{{Type: transaction.HighPriority}}, func(r *result.Invoke, t *transaction.Transaction) error {
		return nil
	}, "param", 1)
	require.NoError(t, err)

	// Bad, it still is a FAULT.
	_, err = a.MakeCall(util.Uint160{}, "method")
	require.Error(t, err)

	// Good.
	client.invRes = &result.Invoke{State: "HALT", GasConsumed: 3, Script: script}
	_, err = a.MakeCall(util.Uint160{}, "method", 1)
	require.NoError(t, err)

	// Tuned.
	opts = Options{
		CheckerModifier: func(r *result.Invoke, t *transaction.Transaction) error {
			t.ValidUntilBlock = 888
			return nil
		},
	}
	at, err = NewTuned(client, a.signers, opts)
	require.NoError(t, err)

	tx, err = at.MakeRun(script)
	require.NoError(t, err)
	require.Equal(t, uint32(888), tx.ValidUntilBlock)
}
