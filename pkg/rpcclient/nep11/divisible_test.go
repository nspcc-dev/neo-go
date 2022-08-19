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

func TestDivisibleBalanceOf(t *testing.T) {
	ta := new(testAct)
	tr := NewDivisibleReader(ta, util.Uint160{1, 2, 3})
	tt := NewDivisible(ta, util.Uint160{1, 2, 3})

	for name, fun := range map[string]func(util.Uint160, []byte) (*big.Int, error){
		"Reader": tr.BalanceOfD,
		"Full":   tt.BalanceOfD,
	} {
		t.Run(name, func(t *testing.T) {
			ta.err = errors.New("")
			_, err := fun(util.Uint160{3, 2, 1}, []byte{1, 2, 3})
			require.Error(t, err)

			ta.err = nil
			ta.res = &result.Invoke{
				State: "HALT",
				Stack: []stackitem.Item{
					stackitem.Make(100500),
				},
			}
			bal, err := fun(util.Uint160{3, 2, 1}, []byte{1, 2, 3})
			require.NoError(t, err)
			require.Equal(t, big.NewInt(100500), bal)

			ta.res = &result.Invoke{
				State: "HALT",
				Stack: []stackitem.Item{
					stackitem.Make([]stackitem.Item{}),
				},
			}
			_, err = fun(util.Uint160{3, 2, 1}, []byte{1, 2, 3})
			require.Error(t, err)
		})
	}
}

func TestDivisibleOwnerOfExpanded(t *testing.T) {
	ta := new(testAct)
	tr := NewDivisibleReader(ta, util.Uint160{1, 2, 3})
	tt := NewDivisible(ta, util.Uint160{1, 2, 3})

	for name, fun := range map[string]func([]byte, int) ([]util.Uint160, error){
		"Reader": tr.OwnerOfExpanded,
		"Full":   tt.OwnerOfExpanded,
	} {
		t.Run(name, func(t *testing.T) {
			ta.err = errors.New("")
			_, err := fun([]byte{1, 2, 3}, 1)
			require.Error(t, err)

			ta.err = nil
			ta.res = &result.Invoke{
				State: "HALT",
				Stack: []stackitem.Item{
					stackitem.Make(100500),
				},
			}
			_, err = fun([]byte{1, 2, 3}, 1)
			require.Error(t, err)

			h := util.Uint160{3, 2, 1}
			ta.res = &result.Invoke{
				State: "HALT",
				Stack: []stackitem.Item{
					stackitem.Make([]stackitem.Item{stackitem.Make(h.BytesBE())}),
				},
			}
			owls, err := fun([]byte{1, 2, 3}, 1)
			require.NoError(t, err)
			require.Equal(t, []util.Uint160{h}, owls)
		})
	}
}

func TestDivisibleOwnerOf(t *testing.T) {
	ta := new(testAct)
	tr := NewDivisibleReader(ta, util.Uint160{1, 2, 3})
	tt := NewDivisible(ta, util.Uint160{1, 2, 3})

	for name, fun := range map[string]func([]byte) (*OwnerIterator, error){
		"Reader": tr.OwnerOf,
		"Full":   tt.OwnerOf,
	} {
		t.Run(name, func(t *testing.T) {
			ta.err = errors.New("")
			_, err := fun([]byte{1})
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
			iter, err := fun([]byte{1})
			require.NoError(t, err)

			ta.res = &result.Invoke{
				Stack: []stackitem.Item{
					stackitem.Make([]stackitem.Item{}),
				},
			}
			_, err = iter.Next(10)
			require.Error(t, err)

			ta.res = &result.Invoke{
				Stack: []stackitem.Item{
					stackitem.Make("not uint160"),
				},
			}
			_, err = iter.Next(10)
			require.Error(t, err)

			h1 := util.Uint160{1, 2, 3}
			h2 := util.Uint160{3, 2, 1}
			ta.res = &result.Invoke{
				Stack: []stackitem.Item{
					stackitem.Make(h1.BytesBE()),
					stackitem.Make(h2.BytesBE()),
				},
			}
			vals, err := iter.Next(10)
			require.NoError(t, err)
			require.Equal(t, []util.Uint160{h1, h2}, vals)

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
							stackitem.Make(h1.BytesBE()),
							stackitem.Make(h2.BytesBE()),
						},
					}),
				},
			}
			iter, err = fun([]byte{1})
			require.NoError(t, err)

			ta.err = errors.New("")
			err = iter.Terminate()
			require.NoError(t, err)
		})
	}
}

func TestDivisibleTransfer(t *testing.T) {
	ta := new(testAct)
	tok := NewDivisible(ta, util.Uint160{1, 2, 3})

	ta.err = errors.New("")
	_, _, err := tok.TransferD(util.Uint160{1, 2, 3}, util.Uint160{3, 2, 1}, big.NewInt(10), []byte{3, 2, 1}, nil)
	require.Error(t, err)

	ta.err = nil
	ta.txh = util.Uint256{1, 2, 3}
	ta.vub = 42
	h, vub, err := tok.TransferD(util.Uint160{1, 2, 3}, util.Uint160{3, 2, 1}, big.NewInt(10), []byte{3, 2, 1}, nil)
	require.NoError(t, err)
	require.Equal(t, ta.txh, h)
	require.Equal(t, ta.vub, vub)

	_, _, err = tok.TransferD(util.Uint160{1, 2, 3}, util.Uint160{3, 2, 1}, big.NewInt(10), []byte{3, 2, 1}, stackitem.NewMap())
	require.Error(t, err)
}

func TestDivisibleTransferTransaction(t *testing.T) {
	ta := new(testAct)
	tok := NewDivisible(ta, util.Uint160{1, 2, 3})

	for _, fun := range []func(from util.Uint160, to util.Uint160, amount *big.Int, id []byte, data interface{}) (*transaction.Transaction, error){
		tok.TransferDTransaction,
		tok.TransferDUnsigned,
	} {
		ta.err = errors.New("")
		_, err := fun(util.Uint160{1, 2, 3}, util.Uint160{3, 2, 1}, big.NewInt(10), []byte{3, 2, 1}, nil)
		require.Error(t, err)

		ta.err = nil
		ta.tx = &transaction.Transaction{Nonce: 100500, ValidUntilBlock: 42}
		tx, err := fun(util.Uint160{1, 2, 3}, util.Uint160{3, 2, 1}, big.NewInt(10), []byte{3, 2, 1}, nil)
		require.NoError(t, err)
		require.Equal(t, ta.tx, tx)

		_, err = fun(util.Uint160{1, 2, 3}, util.Uint160{3, 2, 1}, big.NewInt(10), []byte{3, 2, 1}, stackitem.NewMap())
		require.Error(t, err)
	}
}
