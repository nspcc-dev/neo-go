package compiler_test

import (
	"math/big"
	"testing"
)

var mapTestCases = []testCase{
	{
		"map composite literal",
		`
		package foo
		func Main() int {
			t := map[int]int{
				1: 6,
				2: 9,
			}

			age := t[2]
			return age
		}
		`,
		big.NewInt(9),
	},
	{
		"nested map",
		`
		package foo
		func Main() int {
		t := map[int]map[int]int{
			1: map[int]int{2: 5, 3: 1},
			2: nil,
			5: map[int]int{3: 4, 7: 2},
		}

		x := t[5][3]
		return x
	}
	`,
		big.NewInt(4),
	},
	{
		"map with string index",
		`
		package foo
		func Main() string {
			t := map[string]string{
				"name": "Valera",
				"age": "33",
			}

			name := t["name"]
			return name
		}
		`,
		[]byte("Valera"),
	},
	{
		"delete key",
		`package foo
		func Main() int {
			m := map[int]int{1: 2, 3: 4}
			delete(m, 1)
			return len(m)
		}`,
		big.NewInt(1),
	},
	{
		"delete missing key",
		`package foo
		func Main() int {
			m := map[int]int{3: 4}
			delete(m, 1)
			return len(m)
		}`,
		big.NewInt(1),
	},
}

func TestMaps(t *testing.T) {
	runTestCases(t, mapTestCases)
}
