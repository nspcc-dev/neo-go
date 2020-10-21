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
		"switch with no tag",
		`package main
		func f() bool { return false }
		func Main() int {
			switch {
			case f():
				return 1
			case true:
				return 2
			}
			return 3
		}`,
		big.NewInt(2),
	},
	{
		"type conversion in tag",
		`package main
		type state int
		func Main() int {
			a := 1
			switch state(a) {
			case 1:
				return 42
			default:
				return 11
			}
		}`,
		big.NewInt(42),
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
	{
		"simple fallthrough",
		`package main
		func Main() int {
			n := 2
			switch n {
			case 1: return 5
			case 2: fallthrough
			case 3: return 6
			}
			return 7
		}`,
		big.NewInt(6),
	},
	{
		"double fallthrough",
		`package main
		func Main() int {
			n := 2
			k := 5
			switch n {
			case 0: return k
			case 1: fallthrough
			case 2:
				k++
				fallthrough
			case 3:
			case 4:
				k++
				return k
			}
			return k
		}`,
		big.NewInt(6),
	},
}

func TestSwitch(t *testing.T) {
	for _, tc := range switchTestCases {
		t.Run(tc.name, func(t *testing.T) {
			eval(t, tc.src, tc.result)
		})
	}
}
