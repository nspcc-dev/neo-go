package oracle

import (
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
		{`["Acme Co"]`, "$..Manufacturers[0].Name"},
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
