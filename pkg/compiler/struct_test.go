package compiler_test

import (
	"bytes"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

var structTestCases = []testCase{
	{
		"struct field assign",
		`func F%d() int {
			t := token1 {
				x: 2,
				y: 4,
			}

			age := t.x
			return age
		}

		type token1 struct {
			x int 
			y int
		}
		`,
		big.NewInt(2),
	},
	{
		"struct field from func result",
		`type S struct { x int }
		func fn() int { return 2 }
		func F%d() int {
			t := S{x: fn()}
			return t.x
		}
		`,
		big.NewInt(2),
	},
	{
		"struct field return",
		`type token2 struct {
			x int
			y int
		}

		func F%d() int {
			t := token2 {
				x: 2,
				y: 4,
			}

			return t.x
		}
		`,
		big.NewInt(2),
	},
	{
		"struct field assign",
		`type token3 struct {
			x int
			y int
		}

		func F%d() int {
			t := token3 {
				x: 2,
				y: 4,
			}
			t.x = 10
			return t.x
		}
		`,
		big.NewInt(10),
	},
	{
		"complex struct",
		`type token4 struct {
			x int
			y int
		}

		func F%d() int {
			x := 10

			t := token4 {
				x: 2,
				y: 4,
			}

			y := x + t.x

			return y
		}
		`,
		big.NewInt(12),
	},
	{
		"initialize struct field from variable",
		`type token5 struct {
			x int
			y int
		}

		func F%d() int {
			x := 10
			t := token5 {
				x: x,
				y: 4,
			}
			y := t.x + t.y
			return y
		}
		`,
		big.NewInt(14),
	},
	{
		"assign a variable to a struct field",
		`type token6 struct {
			x int
			y int
		}

		func F%d() int {
			ten := 10
			t := token6 {
				x: 2,
				y: 4,
			}
			t.x = ten
			y := t.y + t.x
			return y
		}
		`,
		big.NewInt(14),
	},
	{
		"increase struct field with +=",
		`type token7 struct { x int }
		func F%d() int {
		t := token7{x: 2}
		t.x += 3
		return t.x
		}
		`,
		big.NewInt(5),
	},
	{
		"assign a struct field to a struct field",
		`type token8 struct {
			x int
			y int
		}

		func F%d() int {
			t1 := token8 {
				x: 2,
				y: 4,
			}
			t2 := token8 {
				x: 3,
				y: 5,
			}
			t1.x = t2.y
			y := t1.x + t2.x
			return y
		}
		`,
		big.NewInt(8),
	},
	{
		"initialize same struct twice",
		`type token9 struct {
			x int
			y int
		}

		func F%d() int {
			t1 := token9 {
				x: 2,
				y: 4,
			}
			t2 := token9 {
				x: 2,
				y: 4,
			}
			return t1.x + t2.y
		}
		`,
		big.NewInt(6),
	},
	{
		"struct methods",
		`type token10 struct {
			x int
		}

		func(t token10) getInteger() int {
			return t.x
		}

		func F%d() int {
			t := token10 {
				x: 4, 
			}
			someInt := t.getInteger()
			return someInt
		}
		`,
		big.NewInt(4),
	},
	{
		"struct methods with arguments",
		`type token11 struct {
			x int
		}

		// Also tests if x conflicts with t.x
		func(t token11) addIntegers(x int, y int) int {
			return t.x + x + y
		}

		func F%d() int {
			t := token11 {
				x: 4, 
			}
			someInt := t.addIntegers(2, 4)
			return someInt
		}
		`,
		big.NewInt(10),
	},
	{
		"initialize struct partially",
		`type token12 struct {
			x int
			y int
			z string
			b bool
		}

		func F%d() int {
			t := token12 {
				x: 4,
			}
			return t.y
		}
		`,
		big.NewInt(0),
	},
	{
		"test return struct from func",
		`type token13 struct {
			x int
			y int
			z string
			b bool
		}

		func newToken() token13 {
			return token13{
				x: 1,
				y: 2, 
				z: "hello",
				b: false,
			}
		}

		func F%d() token13 {
			return newToken()
		}
		`,
		[]stackitem.Item{
			stackitem.NewBigInteger(big.NewInt(1)),
			stackitem.NewBigInteger(big.NewInt(2)),
			stackitem.NewByteArray([]byte("hello")),
			stackitem.NewBool(false),
		},
	},
	{
		"pass struct as argument",
		`type Bar struct {
			amount int
		}

		func addToAmount(x int, bar Bar) int {
			bar.amount = bar.amount + x
			return bar.amount
		}

		func F%d() int {
			b := Bar{
				amount: 10,
			}

			x := addToAmount(4, b)
			return x 
		}
		`,
		big.NewInt(14),
	},
	{
		"declare struct literal",
		`func F%d() int {
			var x struct {
				a int
			}
			x.a = 2
			return x.a
		}
		`,
		big.NewInt(2),
	},
	{
		"declare struct type",
		`type withA struct {
			a int
		}
		func F%d() int {
			var x withA
			x.a = 2
			return x.a
		}
		`,
		big.NewInt(2),
	},
	{
		"nested selectors (simple read)",
		`type S1 struct { x, y S2 }
		type S2 struct { a, b int }
		func F%d() int {
			var s1 S1
			var s2 S2
			s2.a = 3
			s1.y = s2
			return s1.y.a
		}
		`,
		big.NewInt(3),
	},
	{
		"nested selectors (simple write)",
		`type S3 struct { x S4 }
		type S4 struct { a int }
		func F%d() int {
			s1 := S3{
				x: S4 {
					a: 3,
				},
			}
			s1.x.a = 11
			return s1.x.a
		}
		`,
		big.NewInt(11),
	},
	{
		"complex struct default value",
		`type S5 struct { x S6 }
		type S6 struct { y S7 }
		type S7 struct { a int }
		func F%d() int {
			var s1 S5
			s1.x.y.a = 11
			return s1.x.y.a
		}
		`,
		big.NewInt(11),
	},
	{
		"lengthy struct default value",
		`type SS struct { x int; y []byte; z bool }
		func F%d() int {
			var s SS
			return s.x
		}
		`,
		big.NewInt(0),
	},
	{
		"nested selectors (complex write)",
		`type S8 struct { x S9 }
		type S9 struct { y, z S10 }
		type S10 struct { a int }
		func F%d() int {
			var s1 S8
			s1.x.y.a, s1.x.z.a = 11, 31
			return s1.x.y.a + s1.x.z.a
		}
		`,
		big.NewInt(42),
	},
	{
		"omit field names",
		`type pair struct { a, b int }
		func F%d() int {
			p := pair{1, 2}
			x := p.a * 10
			return x + p.b
		}
		`,
		big.NewInt(12),
	},
	{
		"uninitialized struct fields",
		`type Foo struct {
                       i int
                       m map[string]int
                       b []byte
                       a []int
                       s struct { ii int }
               }
               func NewFoo() Foo { return Foo{} }
               func F%d() int {
                       foo := NewFoo()
                       if foo.i != 0 { return 1 }
                       if len(foo.m) != 0 { return 1 }
                       if len(foo.b) != 0 { return 1 }
                       if len(foo.a) != 0 { return 1 }
                       s := foo.s
                       if s.ii != 0 { return 1 }
                       return 2
               }
		`,
		big.NewInt(2),
	},
}

func TestStructs(t *testing.T) {
	srcBuilder := bytes.NewBuffer([]byte("package testcase\n"))
	for i, tc := range structTestCases {
		srcBuilder.WriteString(fmt.Sprintf(tc.src, i))
	}

	ne, di, err := compiler.CompileWithOptions("file.go", strings.NewReader(srcBuilder.String()), nil)
	require.NoError(t, err)

	for i, tc := range structTestCases {
		t.Run(tc.name, func(t *testing.T) {
			v := vm.New()
			invokeMethod(t, fmt.Sprintf("F%d", i), ne.Script, v, di)
			runAndCheck(t, v, tc.result)
		})
	}
}
