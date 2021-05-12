package compiler_test

import (
	"fmt"
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
		big.NewInt(0),
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
		"simple mod",
		`
		package testcase
		func Main() int {
			x := 3 % 2
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
		big.NewInt(0),
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
		big.NewInt(0),
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
		big.NewInt(0),
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
		big.NewInt(0),
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
	{
		"simple add and assign",
		`
		package testcase
		func Main() int {
			x := 2
			x += 1
			return x
		}
		`,
		big.NewInt(3),
	},
	{
		"simple sub and assign",
		`
		package testcase
		func Main() int {
			x := 2
			x -= 1
			return x
		}
		`,
		big.NewInt(1),
	},
	{
		"simple mul and assign",
		`
		package testcase
		func Main() int {
			x := 2
			x *= 2
			return x
		}
		`,
		big.NewInt(4),
	},
	{
		"simple div and assign",
		`
		package testcase
		func Main() int {
			x := 2
			x /= 2
			return x
		}
		`,
		big.NewInt(1),
	},
	{
		"simple mod and assign",
		`
		package testcase
		func Main() int {
			x := 5
			x %= 2
			return x
		}
		`,
		big.NewInt(1),
	},
}

func TestBinaryExprs(t *testing.T) {
	runTestCases(t, binaryExprTestCases)
}

func getBoolExprTestFunc(val bool, cond string) func(t *testing.T) {
	srcTmpl := `package foo
	var s = "str"
	var v = 9
	var cond = %s
	func Main() int {
		if %s {
			return 42
		} %s
		return 17
		%s
	}`
	res := big.NewInt(42)
	if !val {
		res.SetInt64(17)
	}
	return func(t *testing.T) {
		t.Run("AsExpression", func(t *testing.T) {
			src := fmt.Sprintf(srcTmpl, cond, "cond", "", "")
			eval(t, src, res)
		})
		t.Run("InCondition", func(t *testing.T) {
			src := fmt.Sprintf(srcTmpl, "true", cond, "", "")
			eval(t, src, res)
		})
		t.Run("InConditionWithElse", func(t *testing.T) {
			src := fmt.Sprintf(srcTmpl, "true", cond, " else {", "}")
			eval(t, src, res)
		})
	}
}

// TestBooleanExprs enumerates a lot of possible combinations of boolean expressions
// and tests if the result matches to that of Go.
func TestBooleanExprs(t *testing.T) {
	trueExpr := []string{"true", "v < 10", "v <= 9", "v > 8", "v >= 9", "v == 9", "v != 8", `s == "str"`}
	falseExpr := []string{"false", "v > 9", "v >= 10", "v < 9", "v <= 8", "v == 8", "v != 9", `s == "a"`}
	t.Run("Single", func(t *testing.T) {
		for _, s := range trueExpr {
			t.Run(s, getBoolExprTestFunc(true, s))
		}
		for _, s := range falseExpr {
			t.Run(s, getBoolExprTestFunc(false, s))
		}
	})

	type arg struct {
		val bool
		s   string
	}
	t.Run("Combine", func(t *testing.T) {
		var double []arg
		for _, e := range trueExpr {
			double = append(double, arg{true, e + " || false"})
			double = append(double, arg{true, e + " && true"})
		}
		for _, e := range falseExpr {
			double = append(double, arg{false, e + " && true"})
			double = append(double, arg{false, e + " || false"})
		}
		for i := range double {
			t.Run(double[i].s, getBoolExprTestFunc(double[i].val, double[i].s))
		}

		var triple []arg
		for _, a1 := range double {
			for _, a2 := range double {
				triple = append(triple, arg{a1.val || a2.val, fmt.Sprintf("(%s) || (%s)", a1.s, a2.s)})
				triple = append(triple, arg{a1.val && a2.val, fmt.Sprintf("(%s) && (%s)", a1.s, a2.s)})
			}
		}
		for i := range triple {
			t.Run(triple[i].s, getBoolExprTestFunc(triple[i].val, triple[i].s))
		}
	})
}

func TestShortCircuit(t *testing.T) {
	srcTmpl := `package foo
	var a = 1
	func inc() bool { a += 1; return %s }
	func Main() int {
		if %s {
			return 41 + a
		}
		return 16 + a
	}`
	t.Run("||", func(t *testing.T) {
		src := fmt.Sprintf(srcTmpl, "true", "a == 1 || inc()")
		eval(t, src, big.NewInt(42))
	})
	t.Run("&&", func(t *testing.T) {
		src := fmt.Sprintf(srcTmpl, "false", "a == 2 && inc()")
		eval(t, src, big.NewInt(17))
	})
}

func TestEmitBoolean(t *testing.T) {
	src := `package foo
	func Main() int {
		a := true
		if (a == true) == true {
			return 42
		}
		return 11
	}`
	eval(t, src, big.NewInt(42))
}
