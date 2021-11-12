package compiler_test

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

var structTestCases = []testCase{
	{
		"struct field assign",
		`
		package foo
		func Main() int {
			t := token {
				x: 2,
				y: 4,
			}

			age := t.x
			return age
		}

		type token struct {
			x int 
			y int
		}
		`,
		big.NewInt(2),
	},
	{
		"struct field from func result",
		`
		package foo
		type S struct { x int }
		func fn() int { return 2 }
		func Main() int {
			t := S{x: fn()}
			return t.x
		}
		`,
		big.NewInt(2),
	},
	{
		"struct field return",
		`
		package foo
		type token struct {
			x int
			y int
		}

		func Main() int {
			t := token {
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
		`
		package foo
		type token struct {
			x int
			y int
		}

		func Main() int {
			t := token {
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
		`
		package foo
		type token struct {
			x int
			y int
		}

		func Main() int {
			x := 10

			t := token {
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
		`
		package foo
		type token struct {
			x int
			y int
		}

		func Main() int {
			x := 10
			t := token {
				x: x,
				y: 4,
			}
			y := t.x + t.y
			return y
		}`,
		big.NewInt(14),
	},
	{
		"assign a variable to a struct field",
		`
		package foo
		type token struct {
			x int
			y int
		}

		func Main() int {
			ten := 10
			t := token {
				x: 2,
				y: 4,
			}
			t.x = ten
			y := t.y + t.x
			return y
		}`,
		big.NewInt(14),
	},
	{
		"increase struct field with +=",
		`package foo
		type token struct { x int }
		func Main() int {
		t := token{x: 2}
		t.x += 3
		return t.x
		}`,
		big.NewInt(5),
	},
	{
		"assign a struct field to a struct field",
		`
		package foo
		type token struct {
			x int
			y int
		}

		func Main() int {
			t1 := token {
				x: 2,
				y: 4,
			}
			t2 := token {
				x: 3,
				y: 5,
			}
			t1.x = t2.y
			y := t1.x + t2.x
			return y
		}`,
		big.NewInt(8),
	},
	{
		"initialize same struct twice",
		`
		package foo
		type token struct {
			x int
			y int
		}

		func Main() int {
			t1 := token {
				x: 2,
				y: 4,
			}
			t2 := token {
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
		`
		package foo
		type token struct {
			x int
		}

		func(t token) getInteger() int {
			return t.x
		}

		func Main() int {
			t := token {
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
		`
		package foo
		type token struct {
			x int
		}

		// Also tests if x conflicts with t.x
		func(t token) addIntegers(x int, y int) int {
			return t.x + x + y
		}

		func Main() int {
			t := token {
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
		`
		package foo
		type token struct {
			x int
			y int
			z string
			b bool
		}

		func Main() int {
			t := token {
				x: 4,
			}
			return t.y
		}
		`,
		big.NewInt(0),
	},
	{
		"test return struct from func",
		`
		package foo
		type token struct {
			x int
			y int
			z string
			b bool
		}

		func newToken() token {
			return token{
				x: 1,
				y: 2, 
				z: "hello",
				b: false,
			}
		}

		func Main() token {
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
		`
		package foo

		type Bar struct {
			amount int
		}

		func addToAmount(x int, bar Bar) int {
			bar.amount = bar.amount + x
			return bar.amount
		}

		func Main() int {
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
		`package foo
		func Main() int {
			var x struct {
				a int
			}
			x.a = 2
			return x.a
		}`,
		big.NewInt(2),
	},
	{
		"declare struct type",
		`package foo
		type withA struct {
			a int
		}
		func Main() int {
			var x withA
			x.a = 2
			return x.a
		}`,
		big.NewInt(2),
	},
	{
		"nested selectors (simple read)",
		`package foo
		type S1 struct { x, y S2 }
		type S2 struct { a, b int }
		func Main() int {
			var s1 S1
			var s2 S2
			s2.a = 3
			s1.y = s2
			return s1.y.a
		}`,
		big.NewInt(3),
	},
	{
		"nested selectors (simple write)",
		`package foo
		type S1 struct { x S2 }
		type S2 struct { a int }
		func Main() int {
			s1 := S1{
				x: S2 {
					a: 3,
				},
			}
			s1.x.a = 11
			return s1.x.a
		}`,
		big.NewInt(11),
	},
	{
		"complex struct default value",
		`package foo
		type S1 struct { x S2 }
		type S2 struct { y S3 }
		type S3 struct { a int }
		func Main() int {
			var s1 S1
			s1.x.y.a = 11
			return s1.x.y.a
		}`,
		big.NewInt(11),
	},
	{
		"lengthy struct default value",
		`package foo
		type S struct { x int; y []byte; z bool }
		func Main() int {
			var s S
			return s.x
		}`,
		big.NewInt(0),
	},
	{
		"nested selectors (complex write)",
		`package foo
		type S1 struct { x S2 }
		type S2 struct { y, z S3 }
		type S3 struct { a int }
		func Main() int {
			var s1 S1
			s1.x.y.a, s1.x.z.a = 11, 31
			return s1.x.y.a + s1.x.z.a
		}`,
		big.NewInt(42),
	},
	{
		"omit field names",
		`package foo
		type pair struct { a, b int }
		func Main() int {
			p := pair{1, 2}
			x := p.a * 10
			return x + p.b
		}`,
		big.NewInt(12),
	},
	{
		"uninitialized struct fields",
		`package foo
               type Foo struct {
                       i int
                       m map[string]int
                       b []byte
                       a []int
                       s struct { ii int }
               }
               func NewFoo() Foo { return Foo{} }
               func Main() int {
                       foo := NewFoo()
                       if foo.i != 0 { return 1 }
                       if len(foo.m) != 0 { return 1 }
                       if len(foo.b) != 0 { return 1 }
                       if len(foo.a) != 0 { return 1 }
                       s := foo.s
                       if s.ii != 0 { return 1 }
                       return 2
               }`,
		big.NewInt(2),
	},
}

func TestStructs(t *testing.T) {
	runTestCases(t, structTestCases)
}
