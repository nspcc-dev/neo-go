package nns

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func TestRecordStateFromStackItem(t *testing.T) {
	r := &RecordState{}
	require.Error(t, r.FromStackItem(stackitem.Make(42)))
	require.Error(t, r.FromStackItem(stackitem.Make([]stackitem.Item{})))
	require.Error(t, r.FromStackItem(stackitem.Make([]stackitem.Item{
		stackitem.Make([]stackitem.Item{}),
		stackitem.Make(16),
		stackitem.Make("cool"),
	})))
	require.Error(t, r.FromStackItem(stackitem.Make([]stackitem.Item{
		stackitem.Make("n3"),
		stackitem.Make([]stackitem.Item{}),
		stackitem.Make("cool"),
	})))
	require.Error(t, r.FromStackItem(stackitem.Make([]stackitem.Item{
		stackitem.Make("n3"),
		stackitem.Make(16),
		stackitem.Make([]stackitem.Item{}),
	})))
	require.Error(t, r.FromStackItem(stackitem.Make([]stackitem.Item{
		stackitem.Make("n3"),
		stackitem.Make(100500),
		stackitem.Make("cool"),
	})))
	require.NoError(t, r.FromStackItem(stackitem.Make([]stackitem.Item{
		stackitem.Make("n3"),
		stackitem.Make(16),
		stackitem.Make("cool"),
	})))
}
