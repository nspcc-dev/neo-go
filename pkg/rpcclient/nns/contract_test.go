package nns

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

type testAct struct {
	err error
	res *result.Invoke
}

func (t *testAct) Call(contract util.Uint160, operation string, params ...any) (*result.Invoke, error) {
	return t.res, t.err
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

func TestSimpleGetters(t *testing.T) {
	ta := &testAct{}
	nns := NewReader(ta, util.Uint160{1, 2, 3})

	ta.err = errors.New("")
	_, err := nns.GetPrice()
	require.Error(t, err)
	_, err = nns.IsAvailable("nspcc.neo")
	require.Error(t, err)
	_, err = nns.Resolve("nspcc.neo", A)
	require.Error(t, err)

	ta.err = nil
	ta.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make(100500),
		},
	}
	price, err := nns.GetPrice()
	require.NoError(t, err)
	require.Equal(t, int64(100500), price)

	ta.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make(true),
		},
	}
	ava, err := nns.IsAvailable("nspcc.neo")
	require.NoError(t, err)
	require.Equal(t, true, ava)

	ta.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make("some text"),
		},
	}
	txt, err := nns.Resolve("nspcc.neo", TXT)
	require.NoError(t, err)
	require.Equal(t, "some text", txt)
}

func TestGetAllRecords(t *testing.T) {
	ta := &testAct{}
	nns := NewReader(ta, util.Uint160{1, 2, 3})

	ta.err = errors.New("")
	_, err := nns.GetAllRecords("nspcc.neo")
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
	_, err = nns.GetAllRecords("nspcc.neo")
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
	iter, err := nns.GetAllRecords("nspcc.neo")
	require.NoError(t, err)

	require.NoError(t, err)
	ta.res = &result.Invoke{
		Stack: []stackitem.Item{
			stackitem.Make([]stackitem.Item{
				stackitem.Make("n3"),
				stackitem.Make(16),
				stackitem.Make("cool"),
			}),
		},
	}
	vals, err := iter.Next(10)
	require.NoError(t, err)
	require.Equal(t, 1, len(vals))
	require.Equal(t, RecordState{
		Name: "n3",
		Type: TXT,
		Data: "cool",
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
					stackitem.Make("n3"),
					stackitem.Make(16),
					stackitem.Make("cool"),
				},
			}),
		},
	}
	iter, err = nns.GetAllRecords("nspcc.neo")
	require.NoError(t, err)

	ta.err = errors.New("")
	err = iter.Terminate()
	require.NoError(t, err)
}

func TestGetAllRecordsExpanded(t *testing.T) {
	ta := &testAct{}
	nns := NewReader(ta, util.Uint160{1, 2, 3})

	ta.err = errors.New("")
	_, err := nns.GetAllRecordsExpanded("nspcc.neo", 8)
	require.Error(t, err)

	ta.err = nil
	ta.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make(42),
		},
	}
	_, err = nns.GetAllRecordsExpanded("nspcc.neo", 8)
	require.Error(t, err)

	ta.res = &result.Invoke{
		State: "HALT",
		Stack: []stackitem.Item{
			stackitem.Make([]stackitem.Item{
				stackitem.Make([]stackitem.Item{
					stackitem.Make("n3"),
					stackitem.Make(16),
					stackitem.Make("cool"),
				}),
			}),
		},
	}
	vals, err := nns.GetAllRecordsExpanded("nspcc.neo", 8)
	require.NoError(t, err)
	require.Equal(t, 1, len(vals))
	require.Equal(t, RecordState{
		Name: "n3",
		Type: TXT,
		Data: "cool",
	}, vals[0])
}
