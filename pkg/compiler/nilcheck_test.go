package compiler_test

import (
	"math/big"
	"testing"
)

var nilTestCases = []testCase{
	{
		"nil check positive right",
		`
		package foo
		func Main() int {
			var t any
			if t == nil {
				return 1
			}
			return 2
		}
		`,
		big.NewInt(1),
	},
	{
		"nil check negative right",
		`
		package foo
		func Main() int {
			t := []byte{}
			if t == nil {
				return 1
			}
			return 2
		}
		`,
		big.NewInt(2),
	},
	{
		"nil check positive left",
		`
		package foo
		func Main() int {
			var t any
			if nil == t {
				return 1
			}
			return 2
		}
		`,
		big.NewInt(1),
	},
}

func TestNil(t *testing.T) {
	runTestCases(t, nilTestCases)
}
