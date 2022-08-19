package neo

import (
	"errors"
	"math/big"
	"testing"

	"github.com/google/uuid"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

type testAct struct {
	err error
	ser error
	res *result.Invoke
	rre *result.Invoke
	rer error
	tx  *transaction.Transaction
	txh util.Uint256
	vub uint32
	inv *result.Invoke
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
func (t *testAct) MakeCall(contract util.Uint160, method string, params ...interface{}) (*transaction.Transaction, error) {
	return t.tx, t.err
}
func (t *testAct) MakeUnsignedCall(contract util.Uint160, method string, attrs []transaction.Attribute, params ...interface{}) (*transaction.Transaction, error) {
	return t.tx, t.err
}
func (t *testAct) SendCall(contract util.Uint160, method string, params ...interface{}) (util.Uint256, uint32, error) {
	return t.txh, t.vub, t.err
}
func (t *testAct) Run(script []byte) (*result.Invoke, error) {
	return t.rre, t.rer
}
func (t *testAct) MakeUnsignedUncheckedRun(script []byte, sysFee int64, attrs []transaction.Attribute) (*transaction.Transaction, error) {
	return t.tx, t.err
}
func (t *testAct) Sign(tx *transaction.Transaction) error {
	return t.ser
}
func (t *testAct) SignAndSend(tx *transaction.Transaction) (util.Uint256, uint32, error) {
	return t.txh, t.vub, t.err
}
func (t *testAct) CallAndExpandIterator(contract util.Uint160, method string, maxItems int, params ...interface{}) (*result.Invoke, error) {
	return t.inv, t.err
}
func (t *testAct) TerminateSession(sessionID uuid.UUID) error {
	return t.err
}
func (t *testAct) TraverseIterator(sessionID uuid.UUID, iterator *result.Iterator, num int) ([]stackitem.Item, error) {
	return t.res.Stack, t.err
}

func TestGetAccountState(t *testing.T) {
	ta := &testAct{}
	neo := NewReader(ta)

	ta.err = errors.New("")
	_, err := neo.GetAccountState(util.Uint160{})
	require.Error(t, err)

	ta.err = nil
	ta.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make(42),
		},
	}
	_, err = neo.GetAccountState(util.Uint160{})
	require.Error(t, err)

	ta.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Null{},
		},
	}
	st, err := neo.GetAccountState(util.Uint160{})
	require.NoError(t, err)
	require.Nil(t, st)

	ta.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make([]stackitem.Item{
				stackitem.Make(100500),
				stackitem.Make(42),
				stackitem.Null{},
			}),
		},
	}
	st, err = neo.GetAccountState(util.Uint160{})
	require.NoError(t, err)
	require.Equal(t, &state.NEOBalance{
		NEP17Balance: state.NEP17Balance{
			Balance: *big.NewInt(100500),
		},
		BalanceHeight: 42,
	}, st)
}

func TestGetAllCandidates(t *testing.T) {
	ta := &testAct{}
	neo := NewReader(ta)

	ta.err = errors.New("")
	_, err := neo.GetAllCandidates()
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
	_, err = neo.GetAllCandidates()
	require.Error(t, err)

	// Session-based iterator.
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
	iter, err := neo.GetAllCandidates()
	require.NoError(t, err)

	k, err := keys.NewPrivateKey()
	require.NoError(t, err)
	ta.res = &result.Invoke{
		Stack: []stackitem.Item{
			stackitem.Make([]stackitem.Item{
				stackitem.Make(k.PublicKey().Bytes()),
				stackitem.Make(100500),
			}),
		},
	}
	vals, err := iter.Next(10)
	require.NoError(t, err)
	require.Equal(t, 1, len(vals))
	require.Equal(t, result.Validator{
		PublicKey: *k.PublicKey(),
		Votes:     100500,
	}, vals[0])

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
					stackitem.Make(k.PublicKey().Bytes()),
					stackitem.Make(100500),
				},
			}),
		},
	}
	iter, err = neo.GetAllCandidates()
	require.NoError(t, err)

	ta.err = errors.New("")
	err = iter.Terminate()
	require.NoError(t, err)
}

func TestGetCandidates(t *testing.T) {
	ta := &testAct{}
	neo := NewReader(ta)

	ta.err = errors.New("")
	_, err := neo.GetCandidates()
	require.Error(t, err)

	ta.err = nil
	ta.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make([]stackitem.Item{}),
		},
	}
	cands, err := neo.GetCandidates()
	require.NoError(t, err)
	require.Equal(t, 0, len(cands))

	ta.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{stackitem.Make(42)},
	}
	_, err = neo.GetCandidates()
	require.Error(t, err)

	ta.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make([]stackitem.Item{
				stackitem.Make(42),
			}),
		},
	}
	_, err = neo.GetCandidates()
	require.Error(t, err)

	ta.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make([]stackitem.Item{
				stackitem.Make([]stackitem.Item{}),
			}),
		},
	}
	_, err = neo.GetCandidates()
	require.Error(t, err)

	ta.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make([]stackitem.Item{
				stackitem.Make([]stackitem.Item{
					stackitem.Null{},
					stackitem.Null{},
				}),
			}),
		},
	}
	_, err = neo.GetCandidates()
	require.Error(t, err)

	ta.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make([]stackitem.Item{
				stackitem.Make([]stackitem.Item{
					stackitem.Make("some"),
					stackitem.Null{},
				}),
			}),
		},
	}
	_, err = neo.GetCandidates()
	require.Error(t, err)

	k, err := keys.NewPrivateKey()
	require.NoError(t, err)
	ta.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make([]stackitem.Item{
				stackitem.Make([]stackitem.Item{
					stackitem.Make(k.PublicKey().Bytes()),
					stackitem.Null{},
				}),
			}),
		},
	}
	_, err = neo.GetCandidates()
	require.Error(t, err)

	ta.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make([]stackitem.Item{
				stackitem.Make([]stackitem.Item{
					stackitem.Make(k.PublicKey().Bytes()),
					stackitem.Make("canbeabigint"),
				}),
			}),
		},
	}
	_, err = neo.GetCandidates()
	require.Error(t, err)
}

func TestGetKeys(t *testing.T) {
	ta := &testAct{}
	neo := NewReader(ta)

	k, err := keys.NewPrivateKey()
	require.NoError(t, err)

	for _, m := range []func() (keys.PublicKeys, error){neo.GetCommittee, neo.GetNextBlockValidators} {
		ta.err = errors.New("")
		_, err := m()
		require.Error(t, err)

		ta.err = nil
		ta.res = &result.Invoke{
			State: "HALT",
			Stack: []stackitem.Item{
				stackitem.Make([]stackitem.Item{stackitem.Make(k.PublicKey().Bytes())}),
			},
		}
		ks, err := m()
		require.NoError(t, err)
		require.NotNil(t, ks)
		require.Equal(t, 1, len(ks))
		require.Equal(t, k.PublicKey(), ks[0])
	}
}

func TestGetInts(t *testing.T) {
	ta := &testAct{}
	neo := NewReader(ta)

	meth := []func() (int64, error){
		neo.GetGasPerBlock,
		neo.GetRegisterPrice,
	}

	ta.err = errors.New("")
	for _, m := range meth {
		_, err := m()
		require.Error(t, err)
	}

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
}

func TestUnclaimedGas(t *testing.T) {
	ta := &testAct{}
	neo := NewReader(ta)

	ta.err = errors.New("")
	_, err := neo.UnclaimedGas(util.Uint160{}, 100500)
	require.Error(t, err)

	ta.err = nil
	ta.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make([]stackitem.Item{}),
		},
	}
	_, err = neo.UnclaimedGas(util.Uint160{}, 100500)
	require.Error(t, err)

	ta.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make(42),
		},
	}
	val, err := neo.UnclaimedGas(util.Uint160{}, 100500)
	require.NoError(t, err)
	require.Equal(t, big.NewInt(42), val)
}

func TestIntSetters(t *testing.T) {
	ta := new(testAct)
	neo := New(ta)

	meth := []func(int64) (util.Uint256, uint32, error){
		neo.SetGasPerBlock,
		neo.SetRegisterPrice,
	}

	ta.err = errors.New("")
	for _, m := range meth {
		_, _, err := m(42)
		require.Error(t, err)
	}

	ta.err = nil
	ta.txh = util.Uint256{1, 2, 3}
	ta.vub = 42
	for _, m := range meth {
		h, vub, err := m(100)
		require.NoError(t, err)
		require.Equal(t, ta.txh, h)
		require.Equal(t, ta.vub, vub)
	}
}

func TestIntTransactions(t *testing.T) {
	ta := new(testAct)
	neo := New(ta)

	for _, fun := range []func(int64) (*transaction.Transaction, error){
		neo.SetGasPerBlockTransaction,
		neo.SetGasPerBlockUnsigned,
		neo.SetRegisterPriceTransaction,
		neo.SetRegisterPriceUnsigned,
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

func TestVote(t *testing.T) {
	ta := new(testAct)
	neo := New(ta)

	k, err := keys.NewPrivateKey()
	require.NoError(t, err)

	ta.err = errors.New("")
	_, _, err = neo.Vote(util.Uint160{}, nil)
	require.Error(t, err)
	_, _, err = neo.Vote(util.Uint160{}, k.PublicKey())
	require.Error(t, err)
	_, err = neo.VoteTransaction(util.Uint160{}, nil)
	require.Error(t, err)
	_, err = neo.VoteTransaction(util.Uint160{}, k.PublicKey())
	require.Error(t, err)
	_, err = neo.VoteUnsigned(util.Uint160{}, nil)
	require.Error(t, err)
	_, err = neo.VoteUnsigned(util.Uint160{}, k.PublicKey())
	require.Error(t, err)

	ta.err = nil
	ta.txh = util.Uint256{1, 2, 3}
	ta.vub = 42

	h, vub, err := neo.Vote(util.Uint160{}, nil)
	require.NoError(t, err)
	require.Equal(t, ta.txh, h)
	require.Equal(t, ta.vub, vub)
	h, vub, err = neo.Vote(util.Uint160{}, k.PublicKey())
	require.NoError(t, err)
	require.Equal(t, ta.txh, h)
	require.Equal(t, ta.vub, vub)

	ta.tx = &transaction.Transaction{Nonce: 100500, ValidUntilBlock: 42}
	tx, err := neo.VoteTransaction(util.Uint160{}, nil)
	require.NoError(t, err)
	require.Equal(t, ta.tx, tx)
	tx, err = neo.VoteUnsigned(util.Uint160{}, k.PublicKey())
	require.NoError(t, err)
	require.Equal(t, ta.tx, tx)
}

func TestRegisterCandidate(t *testing.T) {
	ta := new(testAct)
	neo := New(ta)

	k, err := keys.NewPrivateKey()
	require.NoError(t, err)
	pk := k.PublicKey()

	ta.rer = errors.New("")
	_, _, err = neo.RegisterCandidate(pk)
	require.Error(t, err)
	_, err = neo.RegisterCandidateTransaction(pk)
	require.Error(t, err)
	_, err = neo.RegisterCandidateUnsigned(pk)
	require.Error(t, err)

	ta.rer = nil
	ta.txh = util.Uint256{1, 2, 3}
	ta.vub = 42
	ta.rre = &result.Invoke{
		GasConsumed: 100500,
	}
	ta.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make(42),
		},
	}

	h, vub, err := neo.RegisterCandidate(pk)
	require.NoError(t, err)
	require.Equal(t, ta.txh, h)
	require.Equal(t, ta.vub, vub)

	ta.tx = &transaction.Transaction{Nonce: 100500, ValidUntilBlock: 42}
	tx, err := neo.RegisterCandidateTransaction(pk)
	require.NoError(t, err)
	require.Equal(t, ta.tx, tx)
	tx, err = neo.RegisterCandidateUnsigned(pk)
	require.NoError(t, err)
	require.Equal(t, ta.tx, tx)

	ta.ser = errors.New("")
	_, err = neo.RegisterCandidateTransaction(pk)
	require.Error(t, err)

	ta.err = errors.New("")
	_, err = neo.RegisterCandidateUnsigned(pk)
	require.Error(t, err)
}

func TestUnregisterCandidate(t *testing.T) {
	ta := new(testAct)
	neo := New(ta)

	k, err := keys.NewPrivateKey()
	require.NoError(t, err)
	pk := k.PublicKey()

	ta.err = errors.New("")
	_, _, err = neo.UnregisterCandidate(pk)
	require.Error(t, err)
	_, err = neo.UnregisterCandidateTransaction(pk)
	require.Error(t, err)
	_, err = neo.UnregisterCandidateUnsigned(pk)
	require.Error(t, err)

	ta.err = nil
	ta.txh = util.Uint256{1, 2, 3}
	ta.vub = 42

	h, vub, err := neo.UnregisterCandidate(pk)
	require.NoError(t, err)
	require.Equal(t, ta.txh, h)
	require.Equal(t, ta.vub, vub)

	ta.tx = &transaction.Transaction{Nonce: 100500, ValidUntilBlock: 42}
	tx, err := neo.UnregisterCandidateTransaction(pk)
	require.NoError(t, err)
	require.Equal(t, ta.tx, tx)
	tx, err = neo.UnregisterCandidateUnsigned(pk)
	require.NoError(t, err)
	require.Equal(t, ta.tx, tx)
}
