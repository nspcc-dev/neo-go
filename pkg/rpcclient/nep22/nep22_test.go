package nep22

import (
	"errors"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
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

func TestUpdate(t *testing.T) {
	ta := new(testAct)
	tok := NewContract(ta, util.Uint160{1, 2, 3})

	ta.err = errors.New("")
	_, _, err := tok.Update([]byte{0x01}, []byte{0x02}, nil)
	require.Error(t, err)

	ta.err = nil
	ta.txh = util.Uint256{9, 9, 9}
	ta.vub = 128
	h, vub, err := tok.Update([]byte{0x01}, []byte{0x02}, nil)
	require.NoError(t, err)
	require.Equal(t, ta.txh, h)
	require.Equal(t, ta.vub, vub)

	ta.err = nil
	ta.txh = util.Uint256{7, 7, 7}
	ta.vub = 256
	h, vub, err = tok.Update(
		[]byte{0x01}, []byte{0x02},
		&tData{someInt: 42, someString: "hello"},
	)
	require.NoError(t, err)
	require.Equal(t, ta.txh, h)
	require.Equal(t, ta.vub, vub)

	_, _, err = tok.Update([]byte{0x01}, []byte{0x02}, stackitem.NewInterop(nil))
	require.Error(t, err)
}

func TestUpdateTransaction(t *testing.T) {
	actor := new(testAct)
	tok := NewContract(actor, util.Uint160{1, 2, 3})

	for name, fn := range map[string]func(nef, mf []byte, data any) (any, error){
		"UpdateTransaction": func(nef, mf []byte, data any) (any, error) {
			return tok.UpdateTransaction(nef, mf, data)
		},
		"UpdateUnsigned": func(nef, mf []byte, data any) (any, error) {
			return tok.UpdateUnsigned(nef, mf, data)
		},
	} {
		t.Run(name, func(t *testing.T) {
			actor.err = errors.New("rpc failed")
			_, err := fn([]byte{0xAA}, []byte{0xBB}, nil)
			require.Error(t, err)

			actor.err = nil
			stubTx := &transaction.Transaction{Nonce: 12345, ValidUntilBlock: 99}
			actor.tx = stubTx
			res, err := fn([]byte{0xAA}, []byte{0xBB}, nil)
			require.NoError(t, err)
			require.Equal(t, stubTx, res)

			actor.err = nil
			actor.tx = &transaction.Transaction{Nonce: 555, ValidUntilBlock: 11}
			res, err = fn([]byte{0xAA}, []byte{0xBB}, &tData{someInt: 1, someString: "world"})
			require.NoError(t, err)
			require.Equal(t, actor.tx, res)

			_, err = fn([]byte{0xAA}, []byte{0xBB}, stackitem.NewInterop(nil))
			require.Error(t, err)
		})
	}
}
