package notary

import (
	"errors"
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

type testAct struct {
	err error
	res *result.Invoke
	tx  *transaction.Transaction
	txh util.Uint256
	vub uint32
}

func (t *testAct) Call(contract util.Uint160, operation string, params ...any) (*result.Invoke, error) {
	return t.res, t.err
}
func (t *testAct) MakeRun(script []byte) (*transaction.Transaction, error) {
	return t.tx, t.err
}
func (t *testAct) MakeUnsignedRun(script []byte, attrs []transaction.Attribute) (*transaction.Transaction, error) {
	return t.tx, t.err
}
func (t *testAct) SendRun(script []byte) (util.Uint256, uint32, error) {
	return t.txh, t.vub, t.err
}
func (t *testAct) MakeCall(contract util.Uint160, method string, params ...any) (*transaction.Transaction, error) {
	return t.tx, t.err
}
func (t *testAct) MakeUnsignedCall(contract util.Uint160, method string, attrs []transaction.Attribute, params ...any) (*transaction.Transaction, error) {
	return t.tx, t.err
}
func (t *testAct) SendCall(contract util.Uint160, method string, params ...any) (util.Uint256, uint32, error) {
	return t.txh, t.vub, t.err
}

func TestBalanceOf(t *testing.T) {
	ta := &testAct{}
	ntr := NewReader(ta)

	ta.err = errors.New("")
	_, err := ntr.BalanceOf(util.Uint160{})
	require.Error(t, err)

	ta.err = nil
	ta.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make(42),
		},
	}
	res, err := ntr.BalanceOf(util.Uint160{})
	require.NoError(t, err)
	require.Equal(t, big.NewInt(42), res)
}

func TestUint32Getters(t *testing.T) {
	ta := &testAct{}
	ntr := NewReader(ta)

	for name, fun := range map[string]func() (uint32, error){
		"ExpirationOf": func() (uint32, error) {
			return ntr.ExpirationOf(util.Uint160{1, 2, 3})
		},
		"GetMaxNotValidBeforeDelta": ntr.GetMaxNotValidBeforeDelta,
	} {
		t.Run(name, func(t *testing.T) {
			ta.err = errors.New("")
			_, err := fun()
			require.Error(t, err)

			ta.err = nil
			ta.res = &result.Invoke{
				State: "HALT",
				Stack: []stackitem.Item{
					stackitem.Make(42),
				},
			}
			res, err := fun()
			require.NoError(t, err)
			require.Equal(t, uint32(42), res)

			ta.res = &result.Invoke{
				State: "HALT",
				Stack: []stackitem.Item{
					stackitem.Make(-1),
				},
			}
			_, err = fun()
			require.Error(t, err)
		})
	}
}

func TestGetNotaryServiceFeePerKey(t *testing.T) {
	ta := &testAct{}
	ntr := NewReader(ta)

	ta.err = errors.New("")
	_, err := ntr.GetNotaryServiceFeePerKey()
	require.Error(t, err)

	ta.err = nil
	ta.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make(42),
		},
	}
	res, err := ntr.GetNotaryServiceFeePerKey()
	require.NoError(t, err)
	require.Equal(t, int64(42), res)
}

func TestTxSenders(t *testing.T) {
	ta := new(testAct)
	ntr := New(ta)

	for name, fun := range map[string]func() (util.Uint256, uint32, error){
		"LockDepositUntil": func() (util.Uint256, uint32, error) {
			return ntr.LockDepositUntil(util.Uint160{1, 2, 3}, 100500)
		},
		"SetMaxNotValidBeforeDelta": func() (util.Uint256, uint32, error) {
			return ntr.SetMaxNotValidBeforeDelta(42)
		},
		"SetNotaryServiceFeePerKey": func() (util.Uint256, uint32, error) {
			return ntr.SetNotaryServiceFeePerKey(100500)
		},
		"Withdraw": func() (util.Uint256, uint32, error) {
			return ntr.Withdraw(util.Uint160{1, 2, 3}, util.Uint160{3, 2, 1})
		},
	} {
		t.Run(name, func(t *testing.T) {
			ta.err = errors.New("")
			_, _, err := fun()
			require.Error(t, err)

			ta.err = nil
			ta.txh = util.Uint256{1, 2, 3}
			ta.vub = 42
			h, vub, err := fun()
			require.NoError(t, err)
			require.Equal(t, ta.txh, h)
			require.Equal(t, ta.vub, vub)
		})
	}
}

func TestTxMakers(t *testing.T) {
	ta := new(testAct)
	ntr := New(ta)

	for name, fun := range map[string]func() (*transaction.Transaction, error){
		"LockDepositUntilTransaction": func() (*transaction.Transaction, error) {
			return ntr.LockDepositUntilTransaction(util.Uint160{1, 2, 3}, 100500)
		},
		"LockDepositUntilUnsigned": func() (*transaction.Transaction, error) {
			return ntr.LockDepositUntilUnsigned(util.Uint160{1, 2, 3}, 100500)
		},
		"SetMaxNotValidBeforeDeltaTransaction": func() (*transaction.Transaction, error) {
			return ntr.SetMaxNotValidBeforeDeltaTransaction(42)
		},
		"SetMaxNotValidBeforeDeltaUnsigned": func() (*transaction.Transaction, error) {
			return ntr.SetMaxNotValidBeforeDeltaUnsigned(42)
		},
		"SetNotaryServiceFeePerKeyTransaction": func() (*transaction.Transaction, error) {
			return ntr.SetNotaryServiceFeePerKeyTransaction(100500)
		},
		"SetNotaryServiceFeePerKeyUnsigned": func() (*transaction.Transaction, error) {
			return ntr.SetNotaryServiceFeePerKeyUnsigned(100500)
		},
		"WithdrawTransaction": func() (*transaction.Transaction, error) {
			return ntr.WithdrawTransaction(util.Uint160{1, 2, 3}, util.Uint160{3, 2, 1})
		},
		"WithdrawUnsigned": func() (*transaction.Transaction, error) {
			return ntr.WithdrawUnsigned(util.Uint160{1, 2, 3}, util.Uint160{3, 2, 1})
		},
	} {
		t.Run(name, func(t *testing.T) {
			ta.err = errors.New("")
			_, err := fun()
			require.Error(t, err)

			ta.err = nil
			ta.tx = &transaction.Transaction{Nonce: 100500, ValidUntilBlock: 42}
			tx, err := fun()
			require.NoError(t, err)
			require.Equal(t, ta.tx, tx)
		})
	}
}
