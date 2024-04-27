package transaction

import (
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestCosignerEncodeDecode(t *testing.T) {
	pk, err := keys.NewPrivateKey()
	require.NoError(t, err)
	expected := &Signer{
		Account:          util.Uint160{1, 2, 3, 4, 5},
		Scopes:           CustomContracts | CustomGroups | Rules,
		AllowedContracts: []util.Uint160{{1, 2, 3, 4}, {6, 7, 8, 9}},
		AllowedGroups:    []*keys.PublicKey{pk.PublicKey()},
		Rules:            []WitnessRule{{Action: WitnessAllow, Condition: ConditionCalledByEntry{}}},
	}
	actual := &Signer{}
	testserdes.EncodeDecodeBinary(t, expected, actual)
}

func TestCosignerMarshallUnmarshallJSON(t *testing.T) {
	expected := &Signer{
		Account:          util.Uint160{1, 2, 3, 4, 5},
		Scopes:           CustomContracts,
		AllowedContracts: []util.Uint160{{1, 2, 3, 4}, {6, 7, 8, 9}},
	}
	actual := &Signer{}
	testserdes.MarshalUnmarshalJSON(t, expected, actual)
}

func TestSignerCopy(t *testing.T) {
	pk, err := keys.NewPrivateKey()
	require.NoError(t, err)
	require.Nil(t, (*Signer)(nil).Copy())

	original := &Signer{
		Account:          util.Uint160{1, 2, 3, 4, 5},
		Scopes:           CustomContracts | CustomGroups | Rules,
		AllowedContracts: []util.Uint160{{1, 2, 3, 4}, {6, 7, 8, 9}},
		AllowedGroups:    keys.PublicKeys{pk.PublicKey()},
		Rules:            []WitnessRule{{Action: WitnessAllow, Condition: ConditionCalledByEntry{}}},
	}

	cp := original.Copy()
	require.NotNil(t, cp, "Copied Signer should not be nil")

	require.Equal(t, original.Account, cp.Account)
	require.Equal(t, original.Scopes, cp.Scopes)

	require.NotSame(t, original.AllowedContracts, cp.AllowedContracts)
	require.Equal(t, original.AllowedContracts, cp.AllowedContracts)

	require.NotSame(t, original.AllowedGroups, cp.AllowedGroups)
	require.Equal(t, original.AllowedGroups, cp.AllowedGroups)

	require.NotSame(t, original.Rules, cp.Rules)
	require.Equal(t, original.Rules, cp.Rules)

	original.AllowedContracts[0][0] = 255
	original.AllowedGroups[0] = nil
	original.Rules[0].Action = WitnessDeny

	require.NotEqual(t, original.AllowedContracts[0][0], cp.AllowedContracts[0][0])
	require.NotEqual(t, original.AllowedGroups[0], cp.AllowedGroups[0])
	require.NotEqual(t, original.Rules[0].Action, cp.Rules[0].Action)
}
