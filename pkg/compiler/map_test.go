package compiler_test

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
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
	{
		"swap map elements",
		`package foo
		func Main() map[string]int {
			m := map[string]int{"a":1, "b":2}
			m["a"], m["b"] = m["b"], m["a"]
			return m
		}
		`,
		[]stackitem.MapElement{
			{
				Key:   stackitem.Make("a"),
				Value: stackitem.Make(2),
			},
			{
				Key:   stackitem.Make("b"),
				Value: stackitem.Make(1),
			},
		},
	},
}

func TestMaps(t *testing.T) {
	runTestCases(t, mapTestCases)
}
