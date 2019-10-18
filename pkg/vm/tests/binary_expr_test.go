package vm_test

import (
	"math/big"
	"testing"
)

var binaryExprTestCases = []testCase{
	{
		"simple add",
		`
		package testcase
		func Main() int {
			x := 2 + 2
			return x
		}
		`,
		big.NewInt(4),
	},
	{
		"simple sub",
		`
		package testcase
		func Main() int {
			x := 2 - 2
			return x
		}
		`,
		[]byte{},
	},
	{
		"simple div",
		`
		package testcase
		func Main() int {
			x := 2 / 2
			return x
		}
		`,
		big.NewInt(1),
	},
	{
		"simple mul",
		`
		package testcase
		func Main() int {
			x := 4 * 2
			return x
		}
		`,
		big.NewInt(8),
	},
	{
		"simple binary expr in return",
		`
		package testcase
		func Main() int {
			x := 2
			return 2 + x
		}
		`,
		big.NewInt(4),
	},
	{
		"complex binary expr",
		`
		package testcase
		func Main() int {
			x := 4
			y := 8
			z := x + 2 + 2 - 8
			return y * z
		}
		`,
		big.NewInt(0),
	},
	{
		"compare not equal strings with eql",
		`
		package testcase
		func Main() int {
			str := "a string"
			if str == "another string" {
				return 1
			}
			return 0
		}
		`,
		[]byte{},
	},
	{
		"compare equal strings with eql",
		`
		package testcase
		func Main() int {
			str := "a string"
			if str == "a string" {
				return 1
			}
			return 0
		}
		`,
		big.NewInt(1),
	},
	{
		"compare not equal strings with neq",
		`
		package testcase
		func Main() int {
			str := "a string"
			if str != "another string" {
				return 1
			}
			return 0
		}
		`,
		big.NewInt(1),
	},
	{
		"compare equal strings with neq",
		`
		package testcase
		func Main() int {
			str := "a string"
			if str != "a string" {
				return 1
			}
			return 0
		}
		`,
		[]byte{},
	},
	{
		"compare equal ints with eql",
		`
		package testcase
		func Main() int {
			x := 10
			if x == 10 {
				return 1
			}
			return 0
		}
		`,
		big.NewInt(1),
	},
	{
		"compare equal ints with neq",
		`
		package testcase
		func Main() int {
			x := 10
			if x != 10 {
				return 1
			}
			return 0
		}
		`,
		[]byte{},
	},
	{
		"compare not equal ints with eql",
		`
		package testcase
		func Main() int {
			x := 11
			if x == 10 {
				return 1
			}
			return 0
		}
		`,
		[]byte{},
	},
	{
		"compare not equal ints with neq",
		`
		package testcase
		func Main() int {
			x := 11
			if x != 10 {
				return 1
			}
			return 0
		}
		`,
		big.NewInt(1),
	},
}

func TestBinaryExprs(t *testing.T) {
	runTestCases(t, binaryExprTestCases)
}
