package oracle

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
func (t *testAct) MakeCall(contract util.Uint160, method string, params ...interface{}) (*transaction.Transaction, error) {
	return t.tx, t.err
}
func (t *testAct) MakeUnsignedCall(contract util.Uint160, method string, attrs []transaction.Attribute, params ...interface{}) (*transaction.Transaction, error) {
	return t.tx, t.err
}
func (t *testAct) SendCall(contract util.Uint160, method string, params ...interface{}) (util.Uint256, uint32, error) {
	return t.txh, t.vub, t.err
}

func TestReader(t *testing.T) {
	ta := new(testAct)
	ora := NewReader(ta)

	ta.err = errors.New("")
	_, err := ora.GetPrice()
	require.Error(t, err)

	ta.err = nil
	ta.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make(42),
		},
	}
	price, err := ora.GetPrice()
	require.NoError(t, err)
	require.Equal(t, big.NewInt(42), price)
}

func TestPriceSetter(t *testing.T) {
	ta := new(testAct)
	ora := New(ta)

	big42 := big.NewInt(42)

	ta.err = errors.New("")
	_, _, err := ora.SetPrice(big42)
	require.Error(t, err)
	_, err = ora.SetPriceTransaction(big42)
	require.Error(t, err)
	_, err = ora.SetPriceUnsigned(big42)
	require.Error(t, err)

	ta.err = nil
	ta.txh = util.Uint256{1, 2, 3}
	ta.vub = 42
	ta.tx = transaction.New([]byte{1, 2, 3}, 100500)

	h, vub, err := ora.SetPrice(big42)
	require.NoError(t, err)
	require.Equal(t, ta.txh, h)
	require.Equal(t, ta.vub, vub)

	tx, err := ora.SetPriceTransaction(big42)
	require.NoError(t, err)
	require.Equal(t, ta.tx, tx)
	tx, err = ora.SetPriceUnsigned(big42)
	require.NoError(t, err)
	require.Equal(t, ta.tx, tx)
}
