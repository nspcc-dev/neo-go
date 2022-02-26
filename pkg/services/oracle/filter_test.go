package oracle

import (
	"fmt"
	"math"
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

// In this test we check that processing doesn't collapse when working with
// recursive unions. Filter consists of `depth` unions each of which contains
// `width` indices. For simplicity (also it is the worst possible case) all
// indices are equal. Thus, the expected JSON size is equal to the size of selected element
// multiplied by `width^depth` plus array brackets and intermediate commas.
func TestFilterOOM(t *testing.T) {
	construct := func(depth int, width int) string {
		data := `$`
		for i := 0; i < depth; i++ {
			data = data + `[0`
			for j := 0; j < width; j++ {
				data = data + `,0`
			}
			data = data + `]`
		}
		return data
	}

	t.Run("big, but good", func(t *testing.T) {
		// 32^3 = 2^15 < 2^16 => good
		data := construct(3, 32)
		fmt.Println(string(data))
		raw, err := filter([]byte("[[[{}]]]"), data)
		require.NoError(t, err)
		fmt.Println(math.Pow(20, 3) * 3)
		fmt.Printf("%d\n%s\n", len(raw), string(raw))
		//require.Equal(t, expected, string(raw))
	})
	t.Run("bad, too big", func(t *testing.T) {
		// 64^4 = 2^24 > 2^16 => bad
		for _, depth := range []int{4, 5, 6} {
			data := construct(depth, 64)
			_, err := filter([]byte("[[[[[[{}]]]]]]"), data)
			require.Error(t, err)
		}
	})
}
