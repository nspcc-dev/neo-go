package rpc

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParam_UnmarshalJSON(t *testing.T) {
	msg := `["str1", 123, ["str2", 3]]`
	expected := Params{
		{
			Type:  stringT,
			Value: "str1",
		},
		{
			Type:  numberT,
			Value: 123,
		},
		{
			Type: arrayT,
			Value: []Param{
				{
					Type:  stringT,
					Value: "str2",
				},
				{
					Type:  numberT,
					Value: 3,
				},
			},
		},
	}

	var ps Params
	require.NoError(t, json.Unmarshal([]byte(msg), &ps))
	require.Equal(t, expected, ps)

	msg = `[{"2": 3}]`
	require.Error(t, json.Unmarshal([]byte(msg), &ps))
}
