package tempstorage

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

type testInv struct {
	err error
	res *result.Invoke
}

func (t *testInv) Call(contract util.Uint160, operation string, params ...any) (*result.Invoke, error) {
	return t.res, t.err
}
func (t *testInv) CallAndExpandIterator(contract util.Uint160, method string, maxItems int, params ...any) (*result.Invoke, error) {
	return t.res, t.err
}
func (t *testInv) TerminateSession(sessionID uuid.UUID) error {
	return t.err
}
func (t *testInv) TraverseIterator(sessionID uuid.UUID, iterator *result.Iterator, num int) ([]stackitem.Item, error) {
	return t.res.Stack, t.err
}

func TestReader(t *testing.T) {
	ti := new(testInv)
	r := NewReader(ti)
	h := util.Uint160{1, 2, 3}
	key := []byte("key")

	ti.err = errors.New("")
	_, err := r.Get(h, key)
	require.Error(t, err)
	_, err = r.GetExpiration(h, key)
	require.Error(t, err)
	_, err = r.Find(h, key, 0)
	require.Error(t, err)
	_, err = r.FindExpanded(h, key, 0, 10)
	require.Error(t, err)

	ti.err = nil
	ti.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{stackitem.Make([]byte("value"))},
	}
	v, err := r.Get(h, key)
	require.NoError(t, err)
	require.Equal(t, []byte("value"), v)

	ti.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{stackitem.Null{}},
	}
	v, err = r.Get(h, key)
	require.NoError(t, err)
	require.Nil(t, v)

	ti.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{stackitem.Make(42)},
	}
	exp, err := r.GetExpiration(h, key)
	require.NoError(t, err)
	require.Equal(t, int64(42), exp)

	// Find: session-less iterator is not allowed.
	iid := uuid.New()
	ti.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{stackitem.NewInterop(result.Iterator{ID: &iid})},
	}
	_, err = r.Find(h, key, 0)
	require.Error(t, err)

	ti.res = &result.Invoke{
		Session: uuid.New(),
		State:   "HALT",
		Stack:   []stackitem.Item{stackitem.NewInterop(result.Iterator{ID: &iid})},
	}
	iter, err := r.Find(h, key, 0)
	require.NoError(t, err)

	ti.res = &result.Invoke{Stack: []stackitem.Item{stackitem.Make([]byte("a"))}}
	items, err := iter.Next(10)
	require.NoError(t, err)
	require.Equal(t, []stackitem.Item{stackitem.Make([]byte("a"))}, items)
	require.NoError(t, iter.Terminate())

	ti.err = errors.New("")
	_, err = iter.Next(1)
	require.Error(t, err)
	require.Error(t, iter.Terminate())

	ti.err = nil
	ti.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{stackitem.NewInterop(result.Iterator{
			Values: []stackitem.Item{stackitem.Make([]byte("a"))},
		})},
	}
	iter, err = r.Find(h, key, 0)
	require.NoError(t, err)
	require.NoError(t, iter.Terminate())

	ti.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{stackitem.Make([]any{[]byte("a"), []byte("b")})},
	}
	items, err = r.FindExpanded(h, key, 0, 10)
	require.NoError(t, err)
	require.Len(t, items, 2)
}
