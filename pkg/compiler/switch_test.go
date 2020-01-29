package compiler_test

import (
	"math/big"
	"testing"
)

var switchTestCases = []testCase{
	{
		"simple switch success",
		`package main
		func Main() int {
			a := 5
			switch a {
			case 5: return 2
			}
			return 1
		}`,
		big.NewInt(2),
	},
	{
		"simple switch fail",
		`package main
		func Main() int {
			a := 6
			switch a {
			case 5:
				return 2
			}
			return 1
		}`,
		big.NewInt(1),
	},
	{
		"multiple cases success",
		`package main
		func Main() int {
			a := 6
			switch a {
			case 5: return 2
			case 6: return 3
			}
			return 1
		}`,
		big.NewInt(3),
	},
	{
		"multiple cases fail",
		`package main
		func Main() int {
			a := 7
			switch a {
			case 5: return 2
			case 6: return 3
			}
			return 1
		}`,
		big.NewInt(1),
	},
	{
		"default case",
		`package main
		func Main() int {
			a := 7
			switch a {
			case 5: return 2
			case 6: return 3
			default: return 4
			}
			return 1
		}`,
		big.NewInt(4),
	},
	{
		"empty case before default",
		`package main
		func Main() int {
			a := 6
			switch a {
			case 5: return 2
			case 6:
			default: return 4
			}
			return 1
		}`,
		big.NewInt(1),
	},
	{
		"expression in case clause",
		`package main
		func Main() int {
			a := 6
			b := 3
			switch a {
			case 5: return 2
			case b*3-3: return 3
			}
			return 1
		}`,
		big.NewInt(3),
	},
	{
		"multiple expressions in case",
		`package main
		func Main() int {
			a := 8
			b := 3
			switch a {
			case 5: return 2
			case b*3-3, 7, 8: return 3
			}
			return 1
		}`,
		big.NewInt(3),
	},
	{
		"string switch",
		`package main
		func Main() int {
			name := "Valera"
			switch name {
			case "Misha": return 2
			case "Katya", "Dima": return 3
			case "Lera", "Valer" + "a": return 4
			}
			return 1
		}`,
		big.NewInt(4),
	},
}

func TestSwitch(t *testing.T) {
	for _, tc := range switchTestCases {
		t.Run(tc.name, func(t *testing.T) {
			eval(t, tc.src, tc.result)
		})
	}
}
