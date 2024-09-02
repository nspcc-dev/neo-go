package oracle

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFilter(t *testing.T) {
	js := `{
  	"Stores": [ "Lambton Quay",	"Willis Street" ],
  	"Manufacturers": [
		{
			"Name": "Acme Co",
			"Products": [
		        { "Name": "Anvil", "Price": 50 }
      		]
    	},
    	{
      		"Name": "Contoso",
      		"Products": [
        		{ "Name": "Elbow Grease", "Price": 99.95 },
        		{ "Name": "Headlight Fluid", "Price": 4 }
      		]
    	}
  	]
}`

	testCases := []struct {
		result, path string
	}{
		{"[]", "$.Name"},
		{`["Acme Co"]`, "$.Manufacturers[0].Name"},
		{`[50]`, "$.Manufacturers[0].Products[0].Price"},
		{`["Elbow Grease"]`, "$.Manufacturers[1].Products[0].Name"},
		{`[{"Name":"Elbow Grease","Price":99.95}]`, "$.Manufacturers[1].Products[0]"},
	}

	for _, tc := range testCases {
		t.Run(tc.path, func(t *testing.T) {
			actual, err := filter([]byte(js), tc.path)
			require.NoError(t, err)
			require.Equal(t, tc.result, string(actual))
		})
	}

	t.Run("not an UTF-8", func(t *testing.T) {
		_, err := filter([]byte{0xFF}, "Manufacturers[0].Name")
		require.Error(t, err)
	})
}

func TestFilterOOM(t *testing.T) {
	construct := func(depth, width int) string {
		data := `$`
		for range depth {
			data = data + `[0`
			for j := 1; j < width; j++ {
				data = data + `,0`
			}
			data = data + `]`
		}
		return data
	}
	t.Run("good", func(t *testing.T) {
		expected := "[" + strings.Repeat("{},", 1023) + "{}]"
		data := construct(2, 32)
		actual, err := filter([]byte("[[{}]]"), data)
		require.NoError(t, err)
		require.JSONEq(t, expected, string(actual))
	})
	t.Run("too big", func(t *testing.T) {
		data := construct(3, 32)
		_, err := filter([]byte("[[[[[[{}]]]]]]"), data)
		require.Error(t, err)
	})
	t.Run("no oom", func(t *testing.T) {
		data := construct(6, 64)
		_, err := filter([]byte("[[[[[[{}]]]]]]"), data)
		require.Error(t, err)
	})
}
