package vm_test

import "math/big"

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
