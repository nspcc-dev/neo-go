package compiler_test

import (
	"math/big"
	"testing"
)

var numericTestCases = []testCase{
	{
		"add",
		`
		package foo
		func Main() int {
			x := 2
			y := 4
			return x + y
		}
		`,
		big.NewInt(6),
	},
}

func TestNumericExprs(t *testing.T) {
	runTestCases(t, numericTestCases)
}
