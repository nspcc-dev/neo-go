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
}

func TestMaps(t *testing.T) {
	runTestCases(t, mapTestCases)
}
