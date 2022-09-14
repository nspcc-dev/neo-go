package smartcontract

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestBuilder(t *testing.T) {
	b := NewBuilder()
	require.Equal(t, 0, b.Len())
	b.InvokeMethod(util.Uint160{1, 2, 3}, "method")
	require.Equal(t, 37, b.Len())
	b.InvokeMethod(util.Uint160{1, 2, 3}, "transfer", util.Uint160{3, 2, 1}, util.Uint160{9, 8, 7}, 100500)
	require.Equal(t, 126, b.Len())
	s, err := b.Script()
	require.NoError(t, err)
	require.NotNil(t, s)
	require.Equal(t, 126, len(s))
	b.Reset()
	require.Equal(t, 0, b.Len())
}
