package rolemgmt

import (
	"errors"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/native/noderoles"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
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

func TestReaderGetDesignatedByRole(t *testing.T) {
	ta := new(testAct)
	rc := NewReader(ta)

	ta.err = errors.New("")
	_, err := rc.GetDesignatedByRole(noderoles.Oracle, 0)
	require.Error(t, err)

	ta.err = nil
	ta.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make(100500),
		},
	}
	_, err = rc.GetDesignatedByRole(noderoles.Oracle, 0)
	require.Error(t, err)

	ta.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Null{},
		},
	}
	_, err = rc.GetDesignatedByRole(noderoles.Oracle, 0)
	require.Error(t, err)

	ta.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make([]stackitem.Item{}),
		},
	}
	nodes, err := rc.GetDesignatedByRole(noderoles.Oracle, 0)
	require.NoError(t, err)
	require.NotNil(t, nodes)
	require.Equal(t, 0, len(nodes))

	ta.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make([]stackitem.Item{stackitem.Null{}}),
		},
	}
	_, err = rc.GetDesignatedByRole(noderoles.Oracle, 0)
	require.Error(t, err)

	ta.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make([]stackitem.Item{stackitem.Make(42)}),
		},
	}
	_, err = rc.GetDesignatedByRole(noderoles.Oracle, 0)
	require.Error(t, err)

	k, err := keys.NewPrivateKey()
	require.NoError(t, err)
	ta.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make([]stackitem.Item{stackitem.Make(k.PublicKey().Bytes())}),
		},
	}
	nodes, err = rc.GetDesignatedByRole(noderoles.Oracle, 0)
	require.NoError(t, err)
	require.NotNil(t, nodes)
	require.Equal(t, 1, len(nodes))
	require.Equal(t, k.PublicKey(), nodes[0])
}

func TestDesignateAsRole(t *testing.T) {
	ta := new(testAct)
	rc := New(ta)

	k, err := keys.NewPrivateKey()
	require.NoError(t, err)
	ks := keys.PublicKeys{k.PublicKey()}

	ta.err = errors.New("")
	_, _, err = rc.DesignateAsRole(noderoles.Oracle, ks)
	require.Error(t, err)

	ta.err = nil
	ta.txh = util.Uint256{1, 2, 3}
	ta.vub = 42
	h, vub, err := rc.DesignateAsRole(noderoles.Oracle, ks)
	require.NoError(t, err)
	require.Equal(t, ta.txh, h)
	require.Equal(t, ta.vub, vub)
}

func TestDesignateAsRoleTransaction(t *testing.T) {
	ta := new(testAct)
	rc := New(ta)

	k, err := keys.NewPrivateKey()
	require.NoError(t, err)
	ks := keys.PublicKeys{k.PublicKey()}

	for _, fun := range []func(r noderoles.Role, pubs keys.PublicKeys) (*transaction.Transaction, error){
		rc.DesignateAsRoleTransaction,
		rc.DesignateAsRoleUnsigned,
	} {
		ta.err = errors.New("")
		_, err := fun(noderoles.P2PNotary, ks)
		require.Error(t, err)

		ta.err = nil
		ta.tx = &transaction.Transaction{Nonce: 100500, ValidUntilBlock: 42}
		tx, err := fun(noderoles.P2PNotary, ks)
		require.NoError(t, err)
		require.Equal(t, ta.tx, tx)
	}
}
