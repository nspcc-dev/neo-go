package neorpc

import (
	"encoding/json"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestSignerWithWitnessMarshalUnmarshalJSON(t *testing.T) {
	s := &SignerWithWitness{
		Signer: transaction.Signer{
			Account:          util.Uint160{1, 2, 3},
			Scopes:           transaction.CalledByEntry | transaction.CustomContracts,
			AllowedContracts: []util.Uint160{{1, 2, 3, 4}},
		},
		Witness: transaction.Witness{
			InvocationScript:   []byte{1, 2, 3},
			VerificationScript: []byte{4, 5, 6},
		},
	}
	testserdes.MarshalUnmarshalJSON(t, s, &SignerWithWitness{})

	// Check marshalling separately to ensure Scopes are marshalled OK.
	expected := `{"account":"0xcadb3dc2faa3ef14a13b619c9a43124755aa2569","scopes":"CalledByEntry, CustomContracts"}`
	acc, err := util.Uint160DecodeStringLE("cadb3dc2faa3ef14a13b619c9a43124755aa2569")
	require.NoError(t, err)
	s = &SignerWithWitness{
		Signer: transaction.Signer{
			Account: acc,
			Scopes:  transaction.CalledByEntry | transaction.CustomContracts,
		},
	}
	actual, err := json.Marshal(s)
	require.NoError(t, err)
	require.Equal(t, expected, string(actual))
}
