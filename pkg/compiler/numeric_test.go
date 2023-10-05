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
	{
		"shift uint64",
		`package foo
		func Main() uint64 {
			return 1 << 63
		}`,
		new(big.Int).SetUint64(1 << 63),
	},
}

func TestNumericExprs(t *testing.T) {
	runTestCases(t, numericTestCases)
}
