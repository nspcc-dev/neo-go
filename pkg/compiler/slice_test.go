package compiler_test

import (
	"math/big"
	"testing"
)

var sliceTestCases = []testCase{
	{
		"constant index",
		`
		package foo
		func Main() int {
			a := []int{0,0}
			a[1] = 42
			return a[1]+0
		}
		`,
		big.NewInt(42),
	},
	{
		"variable index",
		`
		package foo
		func Main() int {
			a := []int{0,0}
			i := 1
			a[i] = 42
			return a[1]+0
		}
		`,
		big.NewInt(42),
	},
	{
		"complex test",
		`
		package foo
		func Main() int {
			a := []int{1,2,3}
			x := a[0]
			a[x] = a[x] + 4
			a[x] = a[x] + a[2]
			return a[1]
		}
		`,
		big.NewInt(9),
	},
	{
		"slice literals with variables",
		`
		package foo
		func Main() int {
			elem := 7
			a := []int{6, elem, 8}
			return a[1]
		}
		`,
		big.NewInt(7),
	},
	{
		"slice literals with expressions",
		`
		package foo
		func Main() int {
			elem := []int{3, 7}
			a := []int{6, elem[1]*2+1, 24}
			return a[1]
		}
		`,
		big.NewInt(15),
	},
}

func TestSliceOperations(t *testing.T) {
	runTestCases(t, sliceTestCases)
}
