package nep11

import (
	"errors"
	"math/big"
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
func (t *testAct) CallAndExpandIterator(contract util.Uint160, method string, maxItems int, params ...interface{}) (*result.Invoke, error) {
	return t.res, t.err
}
func (t *testAct) TerminateSession(sessionID uuid.UUID) error {
	return t.err
}
func (t *testAct) TraverseIterator(sessionID uuid.UUID, iterator *result.Iterator, num int) ([]stackitem.Item, error) {
	return t.res.Stack, t.err
}

func TestReaderBalanceOf(t *testing.T) {
	ta := new(testAct)
	tr := NewBaseReader(ta, util.Uint160{1, 2, 3})

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

func TestReaderProperties(t *testing.T) {
	ta := new(testAct)
	tr := NewBaseReader(ta, util.Uint160{1, 2, 3})

	ta.err = errors.New("")
	_, err := tr.Properties([]byte{3, 2, 1})
	require.Error(t, err)

	ta.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make([]stackitem.Item{}),
		},
	}
	_, err = tr.Properties([]byte{3, 2, 1})
	require.Error(t, err)

	ta.err = nil
	ta.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.NewMap(),
		},
	}
	m, err := tr.Properties([]byte{3, 2, 1})
	require.NoError(t, err)
	require.Equal(t, 0, m.Len())
}

func TestReaderTokensOfExpanded(t *testing.T) {
	ta := new(testAct)
	tr := NewBaseReader(ta, util.Uint160{1, 2, 3})

	for name, fun := range map[string]func(int) ([][]byte, error){
		"Tokens": tr.TokensExpanded,
		"TokensOf": func(n int) ([][]byte, error) {
			return tr.TokensOfExpanded(util.Uint160{1, 2, 3}, n)
		},
	} {
		t.Run(name, func(t *testing.T) {
			ta.err = errors.New("")
			_, err := fun(1)
			require.Error(t, err)

			ta.err = nil
			ta.res = &result.Invoke{
				State: "HALT",
				Stack: []stackitem.Item{
					stackitem.Make(100500),
				},
			}
			_, err = fun(1)
			require.Error(t, err)

			ta.res = &result.Invoke{
				State: "HALT",
				Stack: []stackitem.Item{
					stackitem.Make([]stackitem.Item{stackitem.Make("one")}),
				},
			}
			toks, err := fun(1)
			require.NoError(t, err)
			require.Equal(t, [][]byte{[]byte("one")}, toks)
		})
	}
}

func TestReaderTokensOf(t *testing.T) {
	ta := new(testAct)
	tr := NewBaseReader(ta, util.Uint160{1, 2, 3})

	for name, fun := range map[string]func() (*TokenIterator, error){
		"Tokens": tr.Tokens,
		"TokensOf": func() (*TokenIterator, error) {
			return tr.TokensOf(util.Uint160{1, 2, 3})
		},
	} {
		t.Run(name, func(t *testing.T) {
			ta.err = errors.New("")
			_, err := fun()
			require.Error(t, err)

			iid := uuid.New()
			ta.err = nil
			ta.res = &result.Invoke{
				Session: uuid.New(),
				State:   "HALT",
				Stack: []stackitem.Item{
					stackitem.NewInterop(result.Iterator{
						ID: &iid,
					}),
				},
			}
			iter, err := fun()
			require.NoError(t, err)

			ta.res = &result.Invoke{
				Stack: []stackitem.Item{
					stackitem.Make("one"),
					stackitem.Make([]stackitem.Item{}),
				},
			}
			_, err = iter.Next(10)
			require.Error(t, err)

			ta.res = &result.Invoke{
				Stack: []stackitem.Item{
					stackitem.Make("one"),
					stackitem.Make("two"),
				},
			}
			vals, err := iter.Next(10)
			require.NoError(t, err)
			require.Equal(t, [][]byte{[]byte("one"), []byte("two")}, vals)

			ta.err = errors.New("")
			_, err = iter.Next(1)
			require.Error(t, err)

			err = iter.Terminate()
			require.Error(t, err)

			// Value-based iterator.
			ta.err = nil
			ta.res = &result.Invoke{
				State: "HALT",
				Stack: []stackitem.Item{
					stackitem.NewInterop(result.Iterator{
						Values: []stackitem.Item{
							stackitem.Make("one"),
							stackitem.Make("two"),
						},
					}),
				},
			}
			iter, err = fun()
			require.NoError(t, err)

			ta.err = errors.New("")
			err = iter.Terminate()
			require.NoError(t, err)
		})
	}
}

func TestTokenTransfer(t *testing.T) {
	ta := new(testAct)
	tok := NewBase(ta, util.Uint160{1, 2, 3})

	ta.err = errors.New("")
	_, _, err := tok.Transfer(util.Uint160{3, 2, 1}, []byte{3, 2, 1}, nil)
	require.Error(t, err)

	ta.err = nil
	ta.txh = util.Uint256{1, 2, 3}
	ta.vub = 42
	h, vub, err := tok.Transfer(util.Uint160{3, 2, 1}, []byte{3, 2, 1}, nil)
	require.NoError(t, err)
	require.Equal(t, ta.txh, h)
	require.Equal(t, ta.vub, vub)

	_, _, err = tok.Transfer(util.Uint160{3, 2, 1}, []byte{3, 2, 1}, stackitem.NewMap())
	require.Error(t, err)
}

func TestTokenTransferTransaction(t *testing.T) {
	ta := new(testAct)
	tok := NewBase(ta, util.Uint160{1, 2, 3})

	for _, fun := range []func(to util.Uint160, token []byte, data interface{}) (*transaction.Transaction, error){
		tok.TransferTransaction,
		tok.TransferUnsigned,
	} {
		ta.err = errors.New("")
		_, err := fun(util.Uint160{3, 2, 1}, []byte{3, 2, 1}, nil)
		require.Error(t, err)

		ta.err = nil
		ta.tx = &transaction.Transaction{Nonce: 100500, ValidUntilBlock: 42}
		tx, err := fun(util.Uint160{3, 2, 1}, []byte{3, 2, 1}, nil)
		require.NoError(t, err)
		require.Equal(t, ta.tx, tx)

		_, err = fun(util.Uint160{3, 2, 1}, []byte{3, 2, 1}, stackitem.NewMap())
		require.Error(t, err)
	}
}

func TestUnwrapKnownProperties(t *testing.T) {
	_, err := UnwrapKnownProperties(stackitem.NewMap(), errors.New(""))
	require.Error(t, err)

	m, err := UnwrapKnownProperties(stackitem.NewMap(), nil)
	require.NoError(t, err)
	require.NotNil(t, m)
	require.Equal(t, 0, len(m))

	m, err = UnwrapKnownProperties(stackitem.NewMapWithValue([]stackitem.MapElement{
		{Key: stackitem.Make("some"), Value: stackitem.Make("thing")},
	}), nil)
	require.NoError(t, err)
	require.NotNil(t, m)
	require.Equal(t, 0, len(m))

	m, err = UnwrapKnownProperties(stackitem.NewMapWithValue([]stackitem.MapElement{
		{Key: stackitem.Make([]stackitem.Item{}), Value: stackitem.Make("thing")},
	}), nil)
	require.NoError(t, err)
	require.NotNil(t, m)
	require.Equal(t, 0, len(m))

	_, err = UnwrapKnownProperties(stackitem.NewMapWithValue([]stackitem.MapElement{
		{Key: stackitem.Make("name"), Value: stackitem.Make([]stackitem.Item{})},
	}), nil)
	require.Error(t, err)

	_, err = UnwrapKnownProperties(stackitem.NewMapWithValue([]stackitem.MapElement{
		{Key: stackitem.Make("name"), Value: stackitem.Make([]byte{0xff})},
	}), nil)
	require.Error(t, err)

	m, err = UnwrapKnownProperties(stackitem.NewMapWithValue([]stackitem.MapElement{
		{Key: stackitem.Make("name"), Value: stackitem.Make("thing")},
		{Key: stackitem.Make("description"), Value: stackitem.Make("good NFT")},
	}), nil)
	require.NoError(t, err)
	require.NotNil(t, m)
	require.Equal(t, 2, len(m))
	require.Equal(t, "thing", m["name"])
	require.Equal(t, "good NFT", m["description"])
}
