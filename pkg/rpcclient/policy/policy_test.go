package policy

import (
	"errors"
	"testing"

	"github.com/google/uuid"
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
func (t *testAct) CallAndExpandIterator(contract util.Uint160, method string, maxItems int, params ...any) (*result.Invoke, error) {
	return t.res, t.err
}
func (t *testAct) TerminateSession(sessionID uuid.UUID) error {
	return t.err
}
func (t *testAct) TraverseIterator(sessionID uuid.UUID, iterator *result.Iterator, num int) ([]stackitem.Item, error) {
	return t.res.Stack, t.err
}

func TestReader(t *testing.T) {
	ta := new(testAct)
	pc := NewReader(ta)

	meth := []func() (int64, error){
		pc.GetExecFeeFactor,
		pc.GetExecPicoFeeFactor,
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
	_, err = pc.GetWhitelistFeeContracts()
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
	iid := uuid.New()
	ta.res = &result.Invoke{
		Session: uuid.New(),
		State:   "HALT",
		Stack: []stackitem.Item{
			stackitem.NewInterop(result.Iterator{
				ID: &iid,
			}),
		},
	}
	iter, err := pc.GetWhitelistFeeContracts()
	require.NoError(t, err)
	ta.res = &result.Invoke{
		Stack: []stackitem.Item{
			stackitem.Make(append(util.Uint160{1, 2, 3}.BytesBE(), []byte{0, 0, 0, 1}...)),
		},
	}
	whitelisted, err := iter.Next(10)
	require.NoError(t, err)
	require.Equal(t, []WhitelistedContract{{Hash: util.Uint160{1, 2, 3}, Offset: 1}}, whitelisted)
}

func TestGetBlockedAccounts(t *testing.T) {
	ta := &testAct{}
	man := NewReader(ta)

	ta.err = errors.New("")
	_, err := man.GetBlockedAccounts()
	require.Error(t, err)
	_, err = man.GetBlockedAccountsExpanded(5)
	require.Error(t, err)

	ta.err = nil
	iid := uuid.New()
	ta.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.NewInterop(result.Iterator{
				ID: &iid,
			}),
		},
	}
	_, err = man.GetBlockedAccounts()
	require.Error(t, err)

	sid := uuid.New()
	ta.res = &result.Invoke{
		Session: sid,
		State:   "HALT",
		Stack: []stackitem.Item{
			stackitem.NewInterop(result.Iterator{
				ID: &iid,
			}),
		},
	}
	iter, err := man.GetBlockedAccounts()
	require.NoError(t, err)

	ta.res = &result.Invoke{
		Stack: []stackitem.Item{
			stackitem.Make(util.Uint160{1, 2, 3}),
		},
	}
	vals, err := iter.Next(10)
	require.NoError(t, err)
	require.Equal(t, 1, len(vals))
	require.Equal(t, util.Uint160{1, 2, 3}, vals[0])

	ta.err = errors.New("")
	_, err = iter.Next(1)
	require.Error(t, err)
	err = iter.Terminate()
	require.Error(t, err)

	ta.err = nil
	ta.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.NewInterop(result.Iterator{
				Values: []stackitem.Item{
					stackitem.Make(util.Uint160{1, 2, 3}),
				},
			}),
		},
	}
	iter, err = man.GetBlockedAccounts()
	require.NoError(t, err)

	ta.err = errors.New("")
	err = iter.Terminate()
	require.NoError(t, err)

	ta.err = nil
	ta.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make([]stackitem.Item{
				stackitem.Make(util.Uint160{1, 2, 3}.BytesBE()),
			}),
		},
	}
	expandedVals, err := man.GetBlockedAccountsExpanded(5)
	require.NoError(t, err)
	require.Equal(t, 1, len(expandedVals))
	require.Equal(t, util.Uint160{1, 2, 3}, expandedVals[0])
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

func TestWhitelistedSetters(t *testing.T) {
	ta := new(testAct)
	pc := New(ta)

	ta.err = errors.New("")
	_, _, err := pc.SetWhitelistFeeContract(util.Uint160{}, "someMethod", 1, 0)
	require.Error(t, err)
	_, _, err = pc.RemoveWhitelistFeeContract(util.Uint160{}, "someMethod", 1)
	require.Error(t, err)

	ta.err = nil
	ta.txh = util.Uint256{1, 2, 3}
	ta.vub = 42
	h, vub, err := pc.SetWhitelistFeeContract(util.Uint160{}, "someMethod", 1, 0)
	require.NoError(t, err)
	require.Equal(t, ta.txh, h)
	require.Equal(t, ta.vub, vub)
	h, vub, err = pc.RemoveWhitelistFeeContract(util.Uint160{}, "someMethod", 1)
	require.NoError(t, err)
	require.Equal(t, ta.txh, h)
	require.Equal(t, ta.vub, vub)
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

func TestWhitelistedTransactions(t *testing.T) {
	ta := new(testAct)
	pc := New(ta)

	ta.err = errors.New("")
	_, err := pc.SetWhitelistFeeContractTransaction(util.Uint160{}, "someMethod", 1, 0)
	require.Error(t, err)
	_, err = pc.SetWhitelistFeeContractUnsigned(util.Uint160{}, "someMethod", 1, 0)
	require.Error(t, err)
	_, err = pc.RemoveWhitelistFeeContractTransaction(util.Uint160{}, "someMethod", 1)
	require.Error(t, err)
	_, err = pc.RemoveWhitelistFeeContractUnsigned(util.Uint160{}, "someMethod", 1)
	require.Error(t, err)

	ta.err = nil
	ta.tx = &transaction.Transaction{Nonce: 100500, ValidUntilBlock: 42}
	tx, err := pc.SetWhitelistFeeContractTransaction(util.Uint160{}, "someMethod", 1, 0)
	require.NoError(t, err)
	require.Equal(t, ta.tx, tx)
	tx, err = pc.SetWhitelistFeeContractUnsigned(util.Uint160{}, "someMethod", 1, 0)
	require.NoError(t, err)
	require.Equal(t, ta.tx, tx)
	tx, err = pc.RemoveWhitelistFeeContractTransaction(util.Uint160{}, "someMethod", 1)
	require.NoError(t, err)
	require.Equal(t, ta.tx, tx)
	tx, err = pc.RemoveWhitelistFeeContractUnsigned(util.Uint160{}, "someMethod", 1)
	require.NoError(t, err)
	require.Equal(t, ta.tx, tx)
}

func TestWhitelistFeeChangedEvent_FromStackItem(t *testing.T) {
	var fee int64 = 2
	tcs := []struct {
		name     string
		in       []stackitem.Item
		expected *WhitelistFeeChangedEvent
		err      string
	}{
		{
			name: "nil item",
			in:   nil,
			err:  "nil item",
		},
		{
			name: "wrong number of elements",
			in:   []stackitem.Item{},
			err:  "wrong number of event parameters",
		},
		{
			name: "invalid hash",
			in:   []stackitem.Item{stackitem.NewInterop(nil), stackitem.Make(1), stackitem.Make(2), stackitem.Make(3)},
			err:  "invalid hash",
		},
		{
			name: "failed to unwrap hash",
			in:   []stackitem.Item{stackitem.Make([]byte{1, 2, 3}), stackitem.Make(1), stackitem.Make(2), stackitem.Make(3)},
			err:  "failed to unwrap hash",
		},
		{
			name: "invalid method",
			in:   []stackitem.Item{stackitem.Make(util.Uint160{1, 2, 3}), stackitem.NewInterop(nil), stackitem.Make(2), stackitem.Make(3)},
			err:  "invalid method",
		},
		{
			name: "invalid arg count",
			in:   []stackitem.Item{stackitem.Make(util.Uint160{1, 2, 3}), stackitem.Make("someMethod"), stackitem.NewInterop(nil), stackitem.Make(3)},
			err:  "invalid arg count",
		},
		{
			name: "nil fee",
			in:   []stackitem.Item{stackitem.Make(util.Uint160{1, 2, 3}), stackitem.Make("someMethod"), stackitem.Make(1), stackitem.Null{}},
			expected: &WhitelistFeeChangedEvent{
				Hash:   util.Uint160{1, 2, 3},
				Method: "someMethod",
				ArgCnt: 1,
			},
		},
		{
			name: "non-nil fee",
			in:   []stackitem.Item{stackitem.Make(util.Uint160{1, 2, 3}), stackitem.Make("someMethod"), stackitem.Make(1), stackitem.Make(2)},
			expected: &WhitelistFeeChangedEvent{
				Hash:   util.Uint160{1, 2, 3},
				Method: "someMethod",
				ArgCnt: 1,
				Fee:    &fee,
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			actual := new(WhitelistFeeChangedEvent)
			var in *stackitem.Array
			if tc.in != nil {
				in = stackitem.NewArray(tc.in)
			}
			err := actual.FromStackItem(in)
			if len(tc.err) != 0 {
				require.ErrorContains(t, err, tc.err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expected, actual)
			}
		})
	}
}
