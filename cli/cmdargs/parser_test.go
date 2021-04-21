package cmdargs

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestParseCosigner(t *testing.T) {
	acc := util.Uint160{1, 3, 5, 7}
	testCases := map[string]transaction.Signer{
		acc.StringLE(): {
			Account: acc,
			Scopes:  transaction.CalledByEntry,
		},
		"0x" + acc.StringLE(): {
			Account: acc,
			Scopes:  transaction.CalledByEntry,
		},
		acc.StringLE() + ":Global": {
			Account: acc,
			Scopes:  transaction.Global,
		},
		acc.StringLE() + ":CalledByEntry": {
			Account: acc,
			Scopes:  transaction.CalledByEntry,
		},
		acc.StringLE() + ":None": {
			Account: acc,
			Scopes:  transaction.None,
		},
		acc.StringLE() + ":CalledByEntry,CustomContracts": {
			Account: acc,
			Scopes:  transaction.CalledByEntry | transaction.CustomContracts,
		},
	}
	for s, expected := range testCases {
		actual, err := parseCosigner(s)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	}
	errorCases := []string{
		acc.StringLE() + "0",
		acc.StringLE() + ":Unknown",
		acc.StringLE() + ":Global,CustomContracts",
		acc.StringLE() + ":Global,None",
	}
	for _, s := range errorCases {
		_, err := parseCosigner(s)
		require.Error(t, err)
	}
}
