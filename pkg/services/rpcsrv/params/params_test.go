package params

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/stretchr/testify/require"
)

func TestParamsFromAny(t *testing.T) {
	str := "jajaja"

	ps, err := FromAny([]any{str, smartcontract.Parameter{Type: smartcontract.StringType, Value: str}})
	require.NoError(t, err)
	require.Equal(t, 2, len(ps))

	resStr, err := ps[0].GetString()
	require.NoError(t, err)
	require.Equal(t, resStr, str)

	resFP, err := ps[1].GetFuncParam()
	require.NoError(t, err)
	require.Equal(t, resFP.Type, smartcontract.StringType)
	resStr, err = resFP.Value.GetString()
	require.NoError(t, err)
	require.Equal(t, resStr, str)

	// Invalid item.
	_, err = FromAny([]any{smartcontract.Parameter{Type: smartcontract.IntegerType, Value: str}})
	require.Error(t, err)
}
