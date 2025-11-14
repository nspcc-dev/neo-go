package compiler_test

import (
	"bytes"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/stretchr/testify/require"
)

var switchTestCases = []testCase{
	{
		"simple switch success",
		`func F%d() int {
			a := 5
			switch a {
			case 5: return 2
			}
			return 1
		}
		`,
		big.NewInt(2),
	},
	{
		"switch with no tag",
		`func f() bool { return false }
		func F%d() int {
			switch {
			case f():
				return 1
			case true:
				return 2
			}
			return 3
		}
		`,
		big.NewInt(2),
	},
	{
		"type conversion in tag",
		`type state int
		func F%d() int {
			a := 1
			switch state(a) {
			case 1:
				return 42
			default:
				return 11
			}
		}
		`,
		big.NewInt(42),
	},
	{
		"simple switch fail",
		`func F%d() int {
			a := 6
			switch a {
			case 5:
				return 2
			}
			return 1
		}
		`,
		big.NewInt(1),
	},
	{
		"multiple cases success",
		`func F%d() int {
			a := 6
			switch a {
			case 5: return 2
			case 6: return 3
			}
			return 1
		}
		`,
		big.NewInt(3),
	},
	{
		"multiple cases fail",
		`func F%d() int {
			a := 7
			switch a {
			case 5: return 2
			case 6: return 3
			}
			return 1
		}
		`,
		big.NewInt(1),
	},
	{
		"default case",
		`func F%d() int {
			a := 7
			switch a {
			case 5: return 2
			case 6: return 3
			default: return 4
			}
			return 1
		}
		`,
		big.NewInt(4),
	},
	{
		"empty case before default",
		`func F%d() int {
			a := 6
			switch a {
			case 5: return 2
			case 6:
			default: return 4
			}
			return 1
		}
		`,
		big.NewInt(1),
	},
	{
		"case after first default",
		`func F%d() int {
			a := 5
			switch a {
			default: return 4
			case 5: return 2
			}
		}
		`,
		big.NewInt(2),
	},
	{
		"case after intermediate default",
		`func F%d() int {
			a := 6
			switch a {
			case 5: return 2
			default: return 4
			case 6:
			}
			return 1
		}
		`,
		big.NewInt(1),
	},
	{
		"intermediate default",
		`func F%d() int {
			a := 3
			switch a {
			case 5: return 2
			default: return 4
			case 6:
			}
			return 1
		}
		`,
		big.NewInt(4),
	},
	{
		"expression in case clause",
		`func F%d() int {
			a := 6
			b := 3
			switch a {
			case 5: return 2
			case b*3-3: return 3
			}
			return 1
		}
		`,
		big.NewInt(3),
	},
	{
		"multiple expressions in case",
		`func F%d() int {
			a := 8
			b := 3
			switch a {
			case 5: return 2
			case b*3-3, 7, 8: return 3
			}
			return 1
		}
		`,
		big.NewInt(3),
	},
	{
		"string switch",
		`func F%d() int {
			name := "Valera"
			switch name {
			case "Misha": return 2
			case "Katya", "Dima": return 3
			case "Lera", "Valer" + "a": return 4
			}
			return 1
		}
		`,
		big.NewInt(4),
	},
	{
		"break from switch",
		`func F%d() int {
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
		}
		`,
		big.NewInt(1),
	},
	{
		"break from outer for",
		`func F%d() int {
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
		}
		`,
		big.NewInt(7),
	},
	{
		"continue outer for",
		`func F%d() int {
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
		}
		`,
		big.NewInt(2),
	},
	{
		"simple fallthrough",
		`func F%d() int {
			n := 2
			switch n {
			case 1: return 5
			case 2: fallthrough
			case 3: return 6
			}
			return 7
		}
		`,
		big.NewInt(6),
	},
	{
		"double fallthrough",
		`func F%d() int {
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
		}
		`,
		big.NewInt(6),
	},
	{
		"init assignment",
		`func F%d() int {
			var n int
			switch n = 1; {
			}
			return n
		}
		`,
		big.NewInt(1),
	},
	{
		"init short decl with shadowing",
		`func F%d() int {
			n := 10
			switch n := 1; {
			default:
				if n != 1 {
					return -1
				}
			}
			return n
		}
		`,
		big.NewInt(10),
	},
	{
		"init with multiple assignment",
		`func F%d() int {
			var a, b int
			switch a, b = 2, 3; {
			default:
				return a+b
			}
		}
		`,
		big.NewInt(5),
	},
	{
		"init with multiple assignment, without shadowing",
		`func F%d() int {
			switch a, b := 2, 3; {
			default:
				return a+b
			}
		}
		`,
		big.NewInt(5),
	},
	{
		"init with function call",
		`var g int
		func set(v int) int { g = v; return v }
		func F%d() int {
			g = 0
			switch x := set(7); x {
			case 7:
				return g
			default:
				return -1
			}
		}
		`,
		big.NewInt(7),
	},
	{
		"init evaluation order",
		`var order []int
		func add(v int) int { order = append(order, v); return v }
		func F%d() int {
			order = nil
			switch add(1); add(2) {
			case 2:
				if len(order) == 2 && order[0] == 1 && order[1] == 2 {
					return 42
				}
			}
			return -1
		}
		`,
		big.NewInt(42),
	},
	{
		"init without tag",
		`func F%d() int {
			var x int
			switch x = 7; {
			case x == 7:
				return x
			default:
				return -1
			}
		}
		`,
		big.NewInt(7),
	},
	{
		"init short decl with multi shadowing",
		`func F%d() int {
			a, b := 0, 0
			switch a, b := 3, 4; {
			default:
				return a+b
			}
			return a+b
		}
		`,
		big.NewInt(7),
	},
}

func TestSwitch(t *testing.T) {
	srcBuilder := bytes.NewBuffer([]byte("package testcase\n"))
	for i, tc := range switchTestCases {
		_, err := fmt.Fprintf(srcBuilder, tc.src, i)
		require.NoError(t, err)
	}

	ne, di, err := compiler.CompileWithOptions("file.go", strings.NewReader(srcBuilder.String()), nil)
	require.NoError(t, err)

	for i, tc := range switchTestCases {
		t.Run(tc.name, func(t *testing.T) {
			v := vm.New()
			invokeMethod(t, fmt.Sprintf("F%d", i), ne.Script, v, di)
			runAndCheck(t, v, tc.result)
		})
	}
}
