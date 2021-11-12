package transaction

import (
	"encoding/json"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
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
