package policy

import (
	"errors"
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
func (t *testAct) MakeCall(contract util.Uint160, method string, params ...any) (*transaction.Transaction, error) {
	return t.tx, t.err
}
func (t *testAct) MakeUnsignedCall(contract util.Uint160, method string, attrs []transaction.Attribute, params ...any) (*transaction.Transaction, error) {
	return t.tx, t.err
}
func (t *testAct) SendCall(contract util.Uint160, method string, params ...any) (util.Uint256, uint32, error) {
	return t.txh, t.vub, t.err
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

func TestReader(t *testing.T) {
	ta := new(testAct)
	pc := NewReader(ta)

	meth := []func() (int64, error){
		pc.GetExecFeeFactor,
		pc.GetFeePerByte,
		pc.GetStoragePrice,
		pc.GetMaxValidUntilBlockIncrement,
		pc.GetMillisecondsPerBlock,
	}

	ta.err = errors.New("")
	for _, m := range meth {
		_, err := m()
		require.Error(t, err)
	}
	_, err := pc.IsBlocked(util.Uint160{1, 2, 3})
	require.Error(t, err)
	_, err = pc.GetAttributeFee(transaction.ConflictsT)
	require.Error(t, err)

	ta.err = nil
	ta.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make(42),
		},
	}
	for _, m := range meth {
		val, err := m()
		require.NoError(t, err)
		require.Equal(t, int64(42), val)
	}
	v, err := pc.GetAttributeFee(transaction.ConflictsT)
	require.NoError(t, err)
	require.Equal(t, int64(42), v)
	ta.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make(true),
		},
	}
	val, err := pc.IsBlocked(util.Uint160{1, 2, 3})
	require.NoError(t, err)
	require.True(t, val)
}

func TestIntSetters(t *testing.T) {
	ta := new(testAct)
	pc := New(ta)

	meth := []func(int64) (util.Uint256, uint32, error){
		pc.SetExecFeeFactor,
		pc.SetFeePerByte,
		pc.SetStoragePrice,
		pc.SetMaxValidUntilBlockIncrement,
		pc.SetMillisecondsPerBlock,
	}

	ta.err = errors.New("")
	for _, m := range meth {
		_, _, err := m(42)
		require.Error(t, err)
	}
	_, _, err := pc.SetAttributeFee(transaction.OracleResponseT, 123)
	require.Error(t, err)

	ta.err = nil
	ta.txh = util.Uint256{1, 2, 3}
	ta.vub = 42
	for _, m := range meth {
		h, vub, err := m(100)
		require.NoError(t, err)
		require.Equal(t, ta.txh, h)
		require.Equal(t, ta.vub, vub)
	}
	h, vub, err := pc.SetAttributeFee(transaction.OracleResponseT, 123)
	require.NoError(t, err)
	require.Equal(t, ta.txh, h)
	require.Equal(t, ta.vub, vub)
}

func TestUint160Setters(t *testing.T) {
	ta := new(testAct)
	pc := New(ta)

	meth := []func(util.Uint160) (util.Uint256, uint32, error){
		pc.BlockAccount,
		pc.UnblockAccount,
	}

	ta.err = errors.New("")
	for _, m := range meth {
		_, _, err := m(util.Uint160{})
		require.Error(t, err)
	}

	ta.err = nil
	ta.txh = util.Uint256{1, 2, 3}
	ta.vub = 42
	for _, m := range meth {
		h, vub, err := m(util.Uint160{})
		require.NoError(t, err)
		require.Equal(t, ta.txh, h)
		require.Equal(t, ta.vub, vub)
	}
}

func TestIntTransactions(t *testing.T) {
	ta := new(testAct)
	pc := New(ta)

	for _, fun := range []func(int64) (*transaction.Transaction, error){
		pc.SetExecFeeFactorTransaction,
		pc.SetExecFeeFactorUnsigned,
		pc.SetFeePerByteTransaction,
		pc.SetFeePerByteUnsigned,
		pc.SetStoragePriceTransaction,
		pc.SetStoragePriceUnsigned,
		pc.SetMaxValidUntilBlockIncrementTransaction,
		pc.SetMaxValidUntilBlockIncrementUnsigned,
		pc.SetMillisecondsPerBlockTransaction,
		pc.SetMillisecondsPerBlockUnsigned,
	} {
		ta.err = errors.New("")
		_, err := fun(1)
		require.Error(t, err)

		ta.err = nil
		ta.tx = &transaction.Transaction{Nonce: 100500, ValidUntilBlock: 42}
		tx, err := fun(1)
		require.NoError(t, err)
		require.Equal(t, ta.tx, tx)
	}
}

func TestUint160Transactions(t *testing.T) {
	ta := new(testAct)
	pc := New(ta)

	for _, fun := range []func(util.Uint160) (*transaction.Transaction, error){
		pc.BlockAccountTransaction,
		pc.BlockAccountUnsigned,
		pc.UnblockAccountTransaction,
		pc.UnblockAccountUnsigned,
	} {
		ta.err = errors.New("")
		_, err := fun(util.Uint160{1})
		require.Error(t, err)

		ta.err = nil
		ta.tx = &transaction.Transaction{Nonce: 100500, ValidUntilBlock: 42}
		tx, err := fun(util.Uint160{1})
		require.NoError(t, err)
		require.Equal(t, ta.tx, tx)
	}
}
