package vm_test

import (
	"math/big"

	"github.com/CityOfZion/neo-go/pkg/vm"
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
		[]vm.StackItem{
			vm.NewBigIntegerItem(1),
			vm.NewBigIntegerItem(2),
			vm.NewByteArrayItem([]byte("hello")),
			vm.NewBigIntegerItem(0),
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
}
