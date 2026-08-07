package transaction

import (
	"encoding/json"
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func TestWitnessRuleSerDes(t *testing.T) {
	var b bool
	expected := &WitnessRule{
		Action:    WitnessAllow,
		Condition: (*ConditionBoolean)(&b),
	}
	actual := &WitnessRule{}
	testserdes.EncodeDecodeBinary(t, expected, actual)
}

func TestWitnessRuleSerDesBad(t *testing.T) {
	var b bool
	bad := &WitnessRule{
		Action:    0xff,
		Condition: (*ConditionBoolean)(&b),
	}
	badB, err := testserdes.EncodeBinary(bad)
	require.NoError(t, err)
	err = testserdes.DecodeBinary(badB, &WitnessRule{})
	require.Error(t, err)
}

func TestWitnessRuleJSON(t *testing.T) {
	var b bool
	expected := &WitnessRule{
		Action:    WitnessDeny,
		Condition: (*ConditionBoolean)(&b),
	}
	actual := &WitnessRule{}
	testserdes.MarshalUnmarshalJSON(t, expected, actual)
}

func TestWitnessRuleBadJSON(t *testing.T) {
	var cases = []string{
		`{}`,
		`[]`,
		`{"action":"Allow"}`,
		`{"action":"Unknown","condition":{"type":"Boolean", "expression":true}}`,
		`{"action":"Allow","condition":{"type":"Boolean", "expression":42}}`,
	}
	for i := range cases {
		actual := &WitnessRule{}
		err := json.Unmarshal([]byte(cases[i]), actual)
		require.Errorf(t, err, "case %d, json %s", i, cases[i])
	}
}

func TestWitnessRule_ToStackItem(t *testing.T) {
	var b bool
	for _, act := range []WitnessAction{WitnessDeny, WitnessAllow} {
		expected := stackitem.NewArray([]stackitem.Item{
			stackitem.Make(int64(act)),
			stackitem.Make([]stackitem.Item{
				stackitem.Make(WitnessBoolean),
				stackitem.Make(b),
			}),
		})
		actual := (&WitnessRule{
			Action:    act,
			Condition: (*ConditionBoolean)(&b),
		}).ToStackItem()
		require.Equal(t, expected, actual, act)
	}
}

func TestWitnessRule_ToFromStackItem(t *testing.T) {
	var b bool
	expected := &WitnessRule{
		Action:    WitnessAllow,
		Condition: (*ConditionBoolean)(&b),
	}
	actual := &WitnessRule{}
	require.NoError(t, actual.FromStackItem(expected.ToStackItem()))
	require.Equal(t, expected, actual)
}

func TestWitnessRule_FromStackItemErrors(t *testing.T) {
	goodCond := ConditionCalledByEntry{}.ToStackItem()
	errCases := map[string]stackitem.Item{
		"not an array":        stackitem.Make(1),
		"wrong length":        stackitem.NewArray([]stackitem.Item{stackitem.Make(0)}),
		"action not a number": stackitem.NewArray([]stackitem.Item{stackitem.NewArray(nil), goodCond}),
		"unknown action":      stackitem.NewArray([]stackitem.Item{stackitem.Make(0xff), goodCond}),
		"bad condition":       stackitem.NewArray([]stackitem.Item{stackitem.Make(WitnessAllow), stackitem.Make(1)}),
	}
	for name, errCase := range errCases {
		t.Run(name, func(t *testing.T) {
			w := new(WitnessRule)
			require.Error(t, w.FromStackItem(errCase))
		})
	}
}

func TestWitnessRule_ToSCParameter(t *testing.T) {
	var b = true
	w := &WitnessRule{
		Action:    WitnessAllow,
		Condition: (*ConditionBoolean)(&b),
	}
	prm, err := w.ToSCParameter()
	require.NoError(t, err)
	require.Equal(t, smartcontract.Parameter{Type: smartcontract.ArrayType, Value: []smartcontract.Parameter{
		{Type: smartcontract.IntegerType, Value: big.NewInt(int64(WitnessAllow))},
		{Type: smartcontract.ArrayType, Value: []smartcontract.Parameter{
			{Type: smartcontract.IntegerType, Value: big.NewInt(int64(WitnessBoolean))},
			{Type: smartcontract.BoolType, Value: b},
		}},
	}}, prm)
}

func TestWitnessRule_Copy(t *testing.T) {
	b := true
	wr := &WitnessRule{
		Action:    WitnessDeny,
		Condition: (*ConditionBoolean)(&b),
	}
	copied := wr.Copy()
	require.Equal(t, wr.Action, copied.Action)
	require.Equal(t, wr.Condition, copied.Condition)
	require.NotSame(t, wr.Condition, copied.Condition)
}
