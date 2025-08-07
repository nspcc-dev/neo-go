package nep31

import (
	"errors"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

type testAct struct {
	err error
	tx  *transaction.Transaction
	txh util.Uint256
	vub uint32
}

func (a *testAct) MakeRun(script []byte) (*transaction.Transaction, error) {
	return a.tx, a.err
}
func (a *testAct) MakeUnsignedRun(script []byte, attrs []transaction.Attribute) (*transaction.Transaction, error) {
	return a.tx, a.err
}
func (a *testAct) SendRun(script []byte) (util.Uint256, uint32, error) {
	return a.txh, a.vub, a.err
}

func TestDestroy(t *testing.T) {
	actor := new(testAct)
	tok := NewContract(actor, util.Uint160{1, 2, 3})

	actor.err = errors.New("")
	_, _, err := tok.Destroy()
	require.Error(t, err)

	actor.err = nil
	actor.txh = util.Uint256{5, 5, 5}
	actor.vub = 77
	h, vub, err := tok.Destroy()
	require.NoError(t, err)
	require.Equal(t, actor.txh, h)
	require.Equal(t, actor.vub, vub)
}

func TestDestroyTransaction(t *testing.T) {
	actor := new(testAct)
	tok := NewContract(actor, util.Uint160{1, 2, 3})

	for name, fn := range map[string]func() (any, error){
		"DestroyTransaction": func() (any, error) {
			return tok.DestroyTransaction()
		},
		"DestroyUnsigned": func() (any, error) {
			return tok.DestroyUnsigned()
		},
	} {
		t.Run(name, func(t *testing.T) {
			actor.err = errors.New("")
			_, err := fn()
			require.Error(t, err)

			actor.err = nil
			stubTx := &transaction.Transaction{Nonce: 42, ValidUntilBlock: 100}
			actor.tx = stubTx
			res, err := fn()
			require.NoError(t, err)
			require.Equal(t, stubTx, res)
		})
	}
}
