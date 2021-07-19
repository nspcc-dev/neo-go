package compiler_test

import (
	"math/big"
	"testing"
)

var assignTestCases = []testCase{
	{
		"chain define",
		`
		package foo
		func Main() int {
			x := 4
			y := x
			z := y
			foo := z
			bar := foo
			return bar
		}
		`,
		big.NewInt(4),
	},
	{
		"simple assign",
		`
		package foo
		func Main() int {
			x := 4
			x = 8
			return x
		}
		`,
		big.NewInt(8),
	},
	{
		"add assign",
		`
		package foo
		func Main() int {
			x := 4
			x += 8
			return x
		}
		`,
		big.NewInt(12),
	},
	{
		"sub assign",
		`
		package foo
		func Main() int {
			x := 4
			x -= 2
			return x
		}
		`,
		big.NewInt(2),
	},
	{
		"mul assign",
		`
		package foo
		func Main() int {
			x := 4
			x *= 2
			return x
		}
		`,
		big.NewInt(8),
	},
	{
		"div assign",
		`
		package foo
		func Main() int {
			x := 4
			x /= 2
			return x
		}
		`,
		big.NewInt(2),
	},
	{
		"add assign binary expr",
		`
		package foo
		func Main() int {
			x := 4
			x += 6 + 2
			return x
		}
		`,
		big.NewInt(12),
	},
	{
		"add assign binary expr ident",
		`
		package foo
		func Main() int {
			x := 4
			y := 5
			x += 6 + y
			return x
		}
		`,
		big.NewInt(15),
	},
	{
		"add assign for string",
		`package foo
		func Main() string {
			s := "Hello, "
			s += "world!"
			return s
		}`,
		[]byte("Hello, world!"),
	},
	{
		"decl assign",
		`
		package foo
		func Main() int {
			var x int = 4
			return x
		}
		`,
		big.NewInt(4),
	},
	{
		"multi assign",
		`
		package foo
		func Main() int {
			x, y := 1, 2
			return x + y
		}
		`,
		big.NewInt(3),
	},
}

func TestAssignments(t *testing.T) {
	runTestCases(t, assignTestCases)
}

func TestManyAssignments(t *testing.T) {
	src1 := `package foo
	func Main() int {
		a := 0
	`
	src2 := `return a
	}`

	for i := 0; i < 1024; i++ {
		src1 += "a += 1\n"
	}

	eval(t, src1+src2, big.NewInt(1024))
}
