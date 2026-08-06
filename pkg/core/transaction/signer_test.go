package transaction

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func newTestSigner(t *testing.T) *Signer {
	pk, err := keys.NewPrivateKey()
	require.NoError(t, err)
	pk2, err := keys.NewPrivateKey()
	require.NoError(t, err)
	return &Signer{
		Account:          util.Uint160{1, 2, 3, 4, 5},
		Scopes:           CustomContracts | CustomGroups | Rules,
		AllowedContracts: []util.Uint160{{1, 2, 3, 4}, {6, 7, 8, 9}},
		AllowedGroups:    []*keys.PublicKey{pk.PublicKey()},
		Rules: []WitnessRule{
			{Action: WitnessAllow, Condition: ConditionCalledByEntry{}},
			{Action: WitnessDeny, Condition: &ConditionAnd{
				&ConditionScriptHash{1, 2, 3},
				&ConditionNot{Condition: (*ConditionGroup)(pk2.PublicKey())},
			}},
		},
	}
}

func TestSigner_ToFromStackItem(t *testing.T) {
	expected := newTestSigner(t)
	actual := &Signer{}
	testserdes.ToFromStackItem(t, expected, actual)
}

func TestSigner_FromStackItemErrors(t *testing.T) {
	good := []stackitem.Item{
		stackitem.Make(util.Uint160{1, 2, 3}.BytesBE()),
		stackitem.Make(int64(CustomContracts)),
		stackitem.Make([]stackitem.Item{}),
		stackitem.Make([]stackitem.Item{}),
		stackitem.Make([]stackitem.Item{}),
	}
	errCases := map[string]stackitem.Item{
		"not an array":                      stackitem.Make(1),
		"wrong number of elements":          stackitem.NewArray(good[:4]),
		"account is not a byte string":      stackitem.NewArray(replaceAt(good, 0, stackitem.NewArray(nil))),
		"scopes is not a number":            stackitem.NewArray(replaceAt(good, 1, stackitem.NewArray(nil))),
		"allowed contracts is not an array": stackitem.NewArray(replaceAt(good, 2, stackitem.Make(1))),
		"too many allowed contracts":        stackitem.NewArray(replaceAt(good, 2, stackitem.Make(make([]stackitem.Item, maxSubitems+1)))),
		"bad allowed contract":              stackitem.NewArray(replaceAt(good, 2, stackitem.Make([]stackitem.Item{stackitem.NewArray(nil)}))),
		"allowed groups is not an array":    stackitem.NewArray(replaceAt(good, 3, stackitem.Make(1))),
		"too many allowed groups":           stackitem.NewArray(replaceAt(good, 3, stackitem.Make(make([]stackitem.Item, maxSubitems+1)))),
		"bad allowed group":                 stackitem.NewArray(replaceAt(good, 3, stackitem.Make([]stackitem.Item{stackitem.NewArray(nil)}))),
		"rules is not an array":             stackitem.NewArray(replaceAt(good, 4, stackitem.Make(1))),
		"too many rules":                    stackitem.NewArray(replaceAt(good, 4, stackitem.Make(make([]stackitem.Item, maxSubitems+1)))),
		"bad rule":                          stackitem.NewArray(replaceAt(good, 4, stackitem.Make([]stackitem.Item{stackitem.Make(1)}))),
	}
	for name, errCase := range errCases {
		t.Run(name, func(t *testing.T) {
			s := new(Signer)
			require.Error(t, s.FromStackItem(errCase))
		})
	}
}

// replaceAt returns a copy of items with the element at index i replaced by v.
func replaceAt(items []stackitem.Item, i int, v stackitem.Item) []stackitem.Item {
	cp := make([]stackitem.Item, len(items))
	copy(cp, items)
	cp[i] = v
	return cp
}

func TestSigner_ToSCParameter(t *testing.T) {
	s := newTestSigner(t)
	andCond := s.Rules[1].Condition.(*ConditionAnd)
	notCond := (*andCond)[1].(*ConditionNot)
	groupCond := notCond.Condition.(*ConditionGroup)
	groupBytes := (*keys.PublicKey)(groupCond).Bytes()

	prm, err := s.ToSCParameter()
	require.NoError(t, err)
	require.Equal(t, smartcontract.Parameter{Type: smartcontract.ArrayType, Value: []smartcontract.Parameter{
		{Type: smartcontract.Hash160Type, Value: s.Account},
		{Type: smartcontract.IntegerType, Value: big.NewInt(int64(s.Scopes))},
		{Type: smartcontract.ArrayType, Value: []smartcontract.Parameter{
			{Type: smartcontract.Hash160Type, Value: s.AllowedContracts[0]},
			{Type: smartcontract.Hash160Type, Value: s.AllowedContracts[1]},
		}},
		{Type: smartcontract.ArrayType, Value: []smartcontract.Parameter{
			{Type: smartcontract.PublicKeyType, Value: s.AllowedGroups[0].Bytes()},
		}},
		{Type: smartcontract.ArrayType, Value: []smartcontract.Parameter{
			{Type: smartcontract.ArrayType, Value: []smartcontract.Parameter{
				{Type: smartcontract.IntegerType, Value: big.NewInt(int64(WitnessAllow))},
				{Type: smartcontract.ArrayType, Value: []smartcontract.Parameter{
					{Type: smartcontract.IntegerType, Value: big.NewInt(int64(WitnessCalledByEntry))},
				}},
			}},
			{Type: smartcontract.ArrayType, Value: []smartcontract.Parameter{
				{Type: smartcontract.IntegerType, Value: big.NewInt(int64(WitnessDeny))},
				{Type: smartcontract.ArrayType, Value: []smartcontract.Parameter{
					{Type: smartcontract.IntegerType, Value: big.NewInt(int64(WitnessAnd))},
					{Type: smartcontract.ArrayType, Value: []smartcontract.Parameter{
						{Type: smartcontract.ArrayType, Value: []smartcontract.Parameter{
							{Type: smartcontract.IntegerType, Value: big.NewInt(int64(WitnessScriptHash))},
							{Type: smartcontract.Hash160Type, Value: util.Uint160{1, 2, 3}},
						}},
						{Type: smartcontract.ArrayType, Value: []smartcontract.Parameter{
							{Type: smartcontract.IntegerType, Value: big.NewInt(int64(WitnessNot))},
							{Type: smartcontract.ArrayType, Value: []smartcontract.Parameter{
								{Type: smartcontract.IntegerType, Value: big.NewInt(int64(WitnessGroup))},
								{Type: smartcontract.PublicKeyType, Value: groupBytes},
							}},
						}},
					}},
				}},
			}},
		}},
	}}, prm)
}

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
	require.Equal(t, original.AllowedContracts, cp.AllowedContracts)
	require.Equal(t, original.AllowedGroups, cp.AllowedGroups)
	require.Equal(t, original.Rules, cp.Rules)

	original.AllowedContracts[0][0] = 255
	original.AllowedGroups[0] = nil
	original.Rules[0].Action = WitnessDeny

	require.NotEqual(t, original.AllowedContracts[0][0], cp.AllowedContracts[0][0])
	require.NotEqual(t, original.AllowedGroups[0], cp.AllowedGroups[0])
	require.NotEqual(t, original.Rules[0].Action, cp.Rules[0].Action)
}
