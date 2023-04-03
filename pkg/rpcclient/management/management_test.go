package management

import (
	"errors"
	"math/big"
	"testing"

	"github.com/google/uuid"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
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
	man := NewReader(ta)

	ta.err = errors.New("")
	_, err := man.GetContract(util.Uint160{1, 2, 3})
	require.Error(t, err)
	_, err = man.GetContractByID(1)
	require.Error(t, err)
	_, err = man.GetMinimumDeploymentFee()
	require.Error(t, err)
	_, err = man.HasMethod(util.Uint160{1, 2, 3}, "method", 0)
	require.Error(t, err)

	ta.err = nil
	ta.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make(42),
		},
	}
	_, err = man.GetContract(util.Uint160{1, 2, 3})
	require.Error(t, err)
	_, err = man.GetContractByID(1)
	require.Error(t, err)
	fee, err := man.GetMinimumDeploymentFee()
	require.NoError(t, err)
	require.Equal(t, big.NewInt(42), fee)
	hm, err := man.HasMethod(util.Uint160{1, 2, 3}, "method", 0)
	require.NoError(t, err)
	require.True(t, hm)

	ta.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make(false),
		},
	}
	_, err = man.GetContract(util.Uint160{1, 2, 3})
	require.Error(t, err)
	_, err = man.GetContractByID(1)
	require.Error(t, err)
	hm, err = man.HasMethod(util.Uint160{1, 2, 3}, "method", 0)
	require.NoError(t, err)
	require.False(t, hm)

	ta.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make([]stackitem.Item{}),
		},
	}
	_, err = man.GetContract(util.Uint160{1, 2, 3})
	require.Error(t, err)
	_, err = man.GetContractByID(1)
	require.Error(t, err)

	nefFile, _ := nef.NewFile([]byte{1, 2, 3})
	nefBytes, _ := nefFile.Bytes()
	manif := manifest.DefaultManifest("stack item")
	manifItem, _ := manif.ToStackItem()
	ta.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make([]stackitem.Item{
				stackitem.Make(1),
				stackitem.Make(0),
				stackitem.Make(util.Uint160{1, 2, 3}.BytesBE()),
				stackitem.Make(nefBytes),
				manifItem,
			}),
		},
	}
	cs, err := man.GetContract(util.Uint160{1, 2, 3})
	require.NoError(t, err)
	require.Equal(t, int32(1), cs.ID)
	require.Equal(t, uint16(0), cs.UpdateCounter)
	require.Equal(t, util.Uint160{1, 2, 3}, cs.Hash)
	cs2, err := man.GetContractByID(1)
	require.NoError(t, err)
	require.Equal(t, cs, cs2)
}

func TestGetContractHashes(t *testing.T) {
	ta := &testAct{}
	man := NewReader(ta)

	ta.err = errors.New("")
	_, err := man.GetContractHashes()
	require.Error(t, err)
	_, err = man.GetContractHashesExpanded(5)
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
	_, err = man.GetContractHashes()
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
	iter, err := man.GetContractHashes()
	require.NoError(t, err)

	ta.res = &result.Invoke{
		Stack: []stackitem.Item{
			stackitem.Make([]stackitem.Item{
				stackitem.Make([]byte{0, 0, 0, 1}),
				stackitem.Make(util.Uint160{1, 2, 3}.BytesBE()),
			}),
		},
	}
	vals, err := iter.Next(10)
	require.NoError(t, err)
	require.Equal(t, 1, len(vals))
	require.Equal(t, IDHash{
		ID:   1,
		Hash: util.Uint160{1, 2, 3},
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
				Values: []stackitem.Item{stackitem.NewStruct([]stackitem.Item{
					stackitem.Make([]byte{0, 0, 0, 1}),
					stackitem.Make(util.Uint160{1, 2, 3}.BytesBE()),
				})},
			}),
		},
	}
	iter, err = man.GetContractHashes()
	require.NoError(t, err)

	ta.err = errors.New("")
	err = iter.Terminate()
	require.NoError(t, err)

	// Expanded
	ta.err = nil
	ta.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make([]stackitem.Item{stackitem.Make([]stackitem.Item{
				stackitem.Make([]byte{0, 0, 0, 1}),
				stackitem.Make(util.Uint160{1, 2, 3}.BytesBE()),
			})}),
		},
	}
	vals, err = man.GetContractHashesExpanded(5)
	require.NoError(t, err)
	require.Equal(t, 1, len(vals))
	require.Equal(t, IDHash{
		ID:   1,
		Hash: util.Uint160{1, 2, 3},
	}, vals[0])
}

func TestSetMinimumDeploymentFee(t *testing.T) {
	ta := new(testAct)
	man := New(ta)

	ta.err = errors.New("")
	_, _, err := man.SetMinimumDeploymentFee(big.NewInt(42))
	require.Error(t, err)

	for _, m := range []func(*big.Int) (*transaction.Transaction, error){
		man.SetMinimumDeploymentFeeTransaction,
		man.SetMinimumDeploymentFeeUnsigned,
	} {
		_, err = m(big.NewInt(100))
		require.Error(t, err)
	}

	ta.err = nil
	ta.txh = util.Uint256{1, 2, 3}
	ta.vub = 42

	h, vub, err := man.SetMinimumDeploymentFee(big.NewInt(42))
	require.NoError(t, err)
	require.Equal(t, ta.txh, h)
	require.Equal(t, ta.vub, vub)

	ta.tx = transaction.New([]byte{1, 2, 3}, 100500)
	for _, m := range []func(*big.Int) (*transaction.Transaction, error){
		man.SetMinimumDeploymentFeeTransaction,
		man.SetMinimumDeploymentFeeUnsigned,
	} {
		tx, err := m(big.NewInt(100))
		require.NoError(t, err)
		require.Equal(t, ta.tx, tx)
	}
}

func TestDeploy(t *testing.T) {
	ta := new(testAct)
	man := New(ta)
	nefFile, _ := nef.NewFile([]byte{1, 2, 3})
	manif := manifest.DefaultManifest("stack item")

	ta.err = errors.New("")
	_, _, err := man.Deploy(nefFile, manif, nil)
	require.Error(t, err)

	for _, m := range []func(exe *nef.File, manif *manifest.Manifest, data any) (*transaction.Transaction, error){
		man.DeployTransaction,
		man.DeployUnsigned,
	} {
		_, err = m(nefFile, manif, nil)
		require.Error(t, err)
	}

	ta.err = nil
	ta.txh = util.Uint256{1, 2, 3}
	ta.vub = 42

	h, vub, err := man.Deploy(nefFile, manif, nil)
	require.NoError(t, err)
	require.Equal(t, ta.txh, h)
	require.Equal(t, ta.vub, vub)

	ta.tx = transaction.New([]byte{1, 2, 3}, 100500)
	for _, m := range []func(exe *nef.File, manif *manifest.Manifest, data any) (*transaction.Transaction, error){
		man.DeployTransaction,
		man.DeployUnsigned,
	} {
		tx, err := m(nefFile, manif, nil)
		require.NoError(t, err)
		require.Equal(t, ta.tx, tx)

		_, err = m(nefFile, manif, map[int]int{})
		require.Error(t, err)
	}

	_, _, err = man.Deploy(nefFile, manif, map[int]int{})
	require.Error(t, err)

	_, _, err = man.Deploy(nefFile, manif, 100500)
	require.NoError(t, err)

	nefFile.Compiler = "intentionally very long compiler string that will make NEF code explode on encoding"
	_, _, err = man.Deploy(nefFile, manif, nil)
	require.Error(t, err)

	// Unfortunately, manifest _always_ marshals successfully (or panics).
}

func TestItemsToIDHashesErrors(t *testing.T) {
	for name, input := range map[string][]stackitem.Item{
		"not a struct": {stackitem.Make(1)},
		"wrong length": {stackitem.Make([]stackitem.Item{})},
		"wrong id": {stackitem.Make([]stackitem.Item{
			stackitem.Make([]stackitem.Item{}),
			stackitem.Make(util.Uint160{1, 2, 3}.BytesBE()),
		})},
		"lengthy id": {stackitem.Make([]stackitem.Item{
			stackitem.Make(util.Uint160{1, 2, 3}.BytesBE()),
			stackitem.Make(util.Uint160{1, 2, 3}.BytesBE()),
		})},
		"not a good hash": {stackitem.Make([]stackitem.Item{
			stackitem.Make([]byte{0, 0, 0, 1}),
			stackitem.Make([]stackitem.Item{}),
		})},
		"not a good u160 hash": {stackitem.Make([]stackitem.Item{
			stackitem.Make([]byte{0, 0, 0, 1}),
			stackitem.Make(util.Uint256{1, 2, 3}.BytesBE()),
		})},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := itemsToIDHashes(input)
			require.Error(t, err)
		})
	}
}
