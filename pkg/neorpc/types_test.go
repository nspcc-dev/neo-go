package neorpc

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
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

	t.Run("subitems overflow", func(t *testing.T) {
		checkSubitems := func(t *testing.T, bad any) {
			data, err := json.Marshal(bad)
			require.NoError(t, err)
			err = json.Unmarshal(data, &SignerWithWitness{})

			require.Error(t, err)
			require.Contains(t, err.Error(), fmt.Sprintf("got %d, allowed %d at max", transaction.MaxAttributes+1, transaction.MaxAttributes))
		}

		t.Run("groups", func(t *testing.T) {
			pk, err := keys.NewPrivateKey()
			require.NoError(t, err)
			bad := &SignerWithWitness{
				Signer: transaction.Signer{
					AllowedGroups: make([]*keys.PublicKey, transaction.MaxAttributes+1),
				},
			}
			for i := range bad.AllowedGroups {
				bad.AllowedGroups[i] = pk.PublicKey()
			}

			checkSubitems(t, bad)
		})
		t.Run("contracts", func(t *testing.T) {
			bad := &SignerWithWitness{
				Signer: transaction.Signer{
					AllowedContracts: make([]util.Uint160, transaction.MaxAttributes+1),
				},
			}

			checkSubitems(t, bad)
		})
		t.Run("rules", func(t *testing.T) {
			bad := &SignerWithWitness{
				Signer: transaction.Signer{
					Rules: make([]transaction.WitnessRule, transaction.MaxAttributes+1),
				},
			}
			for i := range bad.Rules {
				bad.Rules[i] = transaction.WitnessRule{
					Action:    transaction.WitnessAllow,
					Condition: &transaction.ConditionScriptHash{},
				}
			}

			checkSubitems(t, bad)
		})
	})
}
