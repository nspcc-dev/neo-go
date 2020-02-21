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
	{
		"break from switch",
		`package main
		func Main() int {
			i := 3
			switch i {
			case 2: return 2
			case 3:
				i = 1
				break
				return 3
			case 4: return 4
			}
			return i
		}`,
		big.NewInt(1),
	},
	{
		"break from outer for",
		`package main
		func Main() int {
			i := 3
			loop:
			for i < 10 {
				i++
				switch i {
				case 5:
					i = 7
					break loop
					return 3
				case 6: return 4
				}
			}
			return i
		}`,
		big.NewInt(7),
	},
	{
		"continue outer for",
		`package main
		func Main() int {
			i := 2
			for i < 10 {
				i++
				switch i {
				case 3:
					i = 7
					continue
				case 4, 5, 6, 7: return 5
				case 8: return 2
				}

				if i == 7 {
					return 6
				}
			}
			return i
		}`,
		big.NewInt(2),
	},
}

func TestSwitch(t *testing.T) {
	for _, tc := range switchTestCases {
		t.Run(tc.name, func(t *testing.T) {
			eval(t, tc.src, tc.result)
		})
	}
}
