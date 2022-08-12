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

func (t *testAct) Call(contract util.Uint160, operation string, params ...interface{}) (*result.Invoke, error) {
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

func TestTokenTransfer(t *testing.T) {
	ta := new(testAct)
	tok := New(ta, util.Uint160{1, 2, 3})

	ta.err = errors.New("")
	_, _, err := tok.Transfer(util.Uint160{3, 2, 1}, util.Uint160{3, 2, 1}, big.NewInt(1), nil)
	require.Error(t, err)

	ta.err = nil
	ta.txh = util.Uint256{1, 2, 3}
	ta.vub = 42
	h, vub, err := tok.Transfer(util.Uint160{3, 2, 1}, util.Uint160{3, 2, 1}, big.NewInt(1), nil)
	require.NoError(t, err)
	require.Equal(t, ta.txh, h)
	require.Equal(t, ta.vub, vub)

	_, _, err = tok.Transfer(util.Uint160{3, 2, 1}, util.Uint160{3, 2, 1}, big.NewInt(1), stackitem.NewMap())
	require.Error(t, err)
}

func TestTokenTransferTransaction(t *testing.T) {
	ta := new(testAct)
	tok := New(ta, util.Uint160{1, 2, 3})

	for _, fun := range []func(from util.Uint160, to util.Uint160, amount *big.Int, data interface{}) (*transaction.Transaction, error){
		tok.TransferTransaction,
		tok.TransferUnsigned,
	} {
		ta.err = errors.New("")
		_, err := fun(util.Uint160{3, 2, 1}, util.Uint160{3, 2, 1}, big.NewInt(1), nil)
		require.Error(t, err)

		ta.err = nil
		ta.tx = &transaction.Transaction{Nonce: 100500, ValidUntilBlock: 42}
		tx, err := fun(util.Uint160{3, 2, 1}, util.Uint160{3, 2, 1}, big.NewInt(1), nil)
		require.NoError(t, err)
		require.Equal(t, ta.tx, tx)

		_, err = fun(util.Uint160{3, 2, 1}, util.Uint160{3, 2, 1}, big.NewInt(1), stackitem.NewMap())
		require.Error(t, err)
	}
}
