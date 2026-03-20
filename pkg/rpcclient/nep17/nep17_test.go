package nep17

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

func TestReaderBalanceOf(t *testing.T) {
	ta := new(testAct)
	tr := NewReader(ta, util.Uint160{1, 2, 3})

	ta.err = errors.New("")
	_, err := tr.BalanceOf(util.Uint160{3, 2, 1})
	require.Error(t, err)

	ta.err = nil
	ta.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make(100500),
		},
	}
	bal, err := tr.BalanceOf(util.Uint160{3, 2, 1})
	require.NoError(t, err)
	require.Equal(t, big.NewInt(100500), bal)

	ta.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make([]stackitem.Item{}),
		},
	}
	_, err = tr.BalanceOf(util.Uint160{3, 2, 1})
	require.Error(t, err)
}

type tData struct {
	someInt    int
	someString string
}

func (d *tData) ToStackItem() (stackitem.Item, error) {
	return stackitem.NewStruct([]stackitem.Item{
		stackitem.Make(d.someInt),
		stackitem.Make(d.someString),
	}), nil
}

func (d *tData) FromStackItem(si stackitem.Item) error {
	panic("TODO")
}

func TestTokenTransfer(t *testing.T) {
	ta := new(testAct)
	tok := New(ta, util.Uint160{1, 2, 3})

	for name, fun := range map[string]func(from util.Uint160, to util.Uint160, amount *big.Int, data any) (util.Uint256, uint32, error){
		"Tranfer": tok.Transfer,
		"MultiTransfer": func(from util.Uint160, to util.Uint160, amount *big.Int, data any) (util.Uint256, uint32, error) {
			return tok.MultiTransfer([]TransferParameters{{from, to, amount, data}, {from, to, amount, data}})
		},
	} {
		t.Run(name, func(t *testing.T) {
			ta.err = errors.New("")
			_, _, err := fun(util.Uint160{3, 2, 1}, util.Uint160{3, 2, 1}, big.NewInt(1), nil)
			require.Error(t, err)

			ta.err = nil
			ta.txh = util.Uint256{1, 2, 3}
			ta.vub = 42
			h, vub, err := fun(util.Uint160{3, 2, 1}, util.Uint160{3, 2, 1}, big.NewInt(1), nil)
			require.NoError(t, err)
			require.Equal(t, ta.txh, h)
			require.Equal(t, ta.vub, vub)

			ta.err = nil
			ta.txh = util.Uint256{1, 2, 3}
			ta.vub = 42
			h, vub, err = fun(util.Uint160{3, 2, 1}, util.Uint160{3, 2, 1}, big.NewInt(1), &tData{
				someInt:    5,
				someString: "ur",
			})
			require.NoError(t, err)
			require.Equal(t, ta.txh, h)
			require.Equal(t, ta.vub, vub)

			_, _, err = fun(util.Uint160{3, 2, 1}, util.Uint160{3, 2, 1}, big.NewInt(1), stackitem.NewInterop(nil))
			require.Error(t, err)
		})
	}
	_, _, err := tok.MultiTransfer(nil)
	require.Error(t, err)
	_, _, err = tok.MultiTransfer([]TransferParameters{})
	require.Error(t, err)
}

func TestTokenTransferTransaction(t *testing.T) {
	ta := new(testAct)
	tok := New(ta, util.Uint160{1, 2, 3})

	for name, fun := range map[string]func(from util.Uint160, to util.Uint160, amount *big.Int, data any) (*transaction.Transaction, error){
		"TransferTransaction": tok.TransferTransaction,
		"TransferUnsigned":    tok.TransferUnsigned,
		"MultiTransferTransaction": func(from util.Uint160, to util.Uint160, amount *big.Int, data any) (*transaction.Transaction, error) {
			return tok.MultiTransferTransaction([]TransferParameters{{from, to, amount, data}, {from, to, amount, data}})
		},
		"MultiTransferUnsigned": func(from util.Uint160, to util.Uint160, amount *big.Int, data any) (*transaction.Transaction, error) {
			return tok.MultiTransferUnsigned([]TransferParameters{{from, to, amount, data}, {from, to, amount, data}})
		},
	} {
		t.Run(name, func(t *testing.T) {
			ta.err = errors.New("")
			_, err := fun(util.Uint160{3, 2, 1}, util.Uint160{3, 2, 1}, big.NewInt(1), nil)
			require.Error(t, err)

			ta.err = nil
			ta.tx = &transaction.Transaction{Nonce: 100500, ValidUntilBlock: 42}
			tx, err := fun(util.Uint160{3, 2, 1}, util.Uint160{3, 2, 1}, big.NewInt(1), nil)
			require.NoError(t, err)
			require.Equal(t, ta.tx, tx)

			ta.err = nil
			ta.tx = &transaction.Transaction{Nonce: 100500, ValidUntilBlock: 42}
			tx, err = fun(util.Uint160{3, 2, 1}, util.Uint160{3, 2, 1}, big.NewInt(1), &tData{
				someInt:    5,
				someString: "ur",
			})
			require.NoError(t, err)
			require.Equal(t, ta.tx, tx)

			_, err = fun(util.Uint160{3, 2, 1}, util.Uint160{3, 2, 1}, big.NewInt(1), stackitem.NewInterop(nil))
			require.Error(t, err)
		})
	}
	_, err := tok.MultiTransferTransaction(nil)
	require.Error(t, err)
	_, err = tok.MultiTransferTransaction([]TransferParameters{})
	require.Error(t, err)
	_, err = tok.MultiTransferUnsigned(nil)
	require.Error(t, err)
	_, err = tok.MultiTransferUnsigned([]TransferParameters{})
	require.Error(t, err)
}

func TestTransferEvent_FromStackItem(t *testing.T) {
	t.Run("good", func(t *testing.T) {
		addr := util.Uint160{1, 2, 3}
		for _, tc := range []struct {
			name     string
			in       stackitem.Item
			expected *TransferEvent
		}{
			{"good",
				stackitem.NewArray([]stackitem.Item{stackitem.Make(addr), stackitem.Make(addr), stackitem.Make(1)}),
				&TransferEvent{From: addr, To: addr, Amount: big.NewInt(1)}},
			{
				"nil from",
				stackitem.NewArray([]stackitem.Item{stackitem.Null{}, stackitem.Make(addr), stackitem.Make(1)}),
				&TransferEvent{To: addr, Amount: big.NewInt(1)},
			},
			{
				"nil to",
				stackitem.NewArray([]stackitem.Item{stackitem.Make(addr), stackitem.Null{}, stackitem.Make(1)}),
				&TransferEvent{From: addr, Amount: big.NewInt(1)},
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				actual := new(TransferEvent)
				require.NoError(t, actual.FromStackItem(tc.in))
				require.Equal(t, tc.expected, actual)
			})
		}
	})
	t.Run("bad", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			in   stackitem.Item
			err  string
		}{
			{"nil item", nil, "nil item"},
			{"not an array", stackitem.Make(1), "not an array"},
			{"wrong number of params", stackitem.NewArray([]stackitem.Item{}), "wrong number of event parameters"},
			{"invalid from", stackitem.NewArray([]stackitem.Item{stackitem.NewInterop(nil), stackitem.Null{}, stackitem.Make(1)}), "invalid From"},
			{"failed to decode from from", stackitem.NewArray([]stackitem.Item{stackitem.NewByteArray([]byte{1, 2, 3}), stackitem.Null{}, stackitem.Make(1)}), "failed to decode From"},
			{"invalid to", stackitem.NewArray([]stackitem.Item{stackitem.Null{}, stackitem.NewInterop(nil), stackitem.Make(1)}), "invalid To"},
			{"failed to decode to", stackitem.NewArray([]stackitem.Item{stackitem.Null{}, stackitem.NewByteArray([]byte{1, 2, 3}), stackitem.Make(1)}), "failed to decode To"},
			{"invalid amount", stackitem.NewArray([]stackitem.Item{stackitem.Null{}, stackitem.Null{}, stackitem.NewInterop(nil)}), "failed to decode Amount"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				actual := new(TransferEvent)
				require.ErrorContains(t, actual.FromStackItem(tc.in), tc.err)
			})
		}
	})
}
