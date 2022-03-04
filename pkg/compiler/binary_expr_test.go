package compiler_test

import (
	"bytes"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/stretchr/testify/require"
)

var binaryExprTestCases = []testCase{
	{
		"simple add",
		`func F%d() int {
			x := 2 + 2
			return x
		}
		`,
		big.NewInt(4),
	},
	{
		"simple sub",
		`func F%d() int {
			x := 2 - 2
			return x
		}
		`,
		big.NewInt(0),
	},
	{
		"simple div",
		`func F%d() int {
			x := 2 / 2
			return x
		}
		`,
		big.NewInt(1),
	},
	{
		"simple mod",
		`func F%d() int {
			x := 3 %% 2
			return x
		}
		`,
		big.NewInt(1),
	},
	{
		"simple mul",
		`func F%d() int {
			x := 4 * 2
			return x
		}
		`,
		big.NewInt(8),
	},
	{
		"simple binary expr in return",
		`func F%d() int {
			x := 2
			return 2 + x
		}
		`,
		big.NewInt(4),
	},
	{
		"complex binary expr",
		`func F%d() int {
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
		`func F%d() int {
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
		`func F%d() int {
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
		`func F%d() int {
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
		`func F%d() int {
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
		`func F%d() int {
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
		`func F%d() int {
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
		`func F%d() int {
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
		`func F%d() int {
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
		`func F%d() int {
			x := 2
			x += 1
			return x
		}
		`,
		big.NewInt(3),
	},
	{
		"simple sub and assign",
		`func F%d() int {
			x := 2
			x -= 1
			return x
		}
		`,
		big.NewInt(1),
	},
	{
		"simple mul and assign",
		`func F%d() int {
			x := 2
			x *= 2
			return x
		}
		`,
		big.NewInt(4),
	},
	{
		"simple div and assign",
		`func F%d() int {
			x := 2
			x /= 2
			return x
		}
		`,
		big.NewInt(1),
	},
	{
		"simple mod and assign",
		`func F%d() int {
			x := 5
			x %%= 2
			return x
		}
		`,
		big.NewInt(1),
	},
}

func TestBinaryExprs(t *testing.T) {
	srcBuilder := bytes.NewBuffer([]byte("package testcase\n"))
	for i, tc := range binaryExprTestCases {
		srcBuilder.WriteString(fmt.Sprintf(tc.src, i))
	}

	ne, di, err := compiler.CompileWithOptions("file.go", strings.NewReader(srcBuilder.String()), nil)
	require.NoError(t, err)

	for i, tc := range binaryExprTestCases {
		v := vm.New()
		t.Run(tc.name, func(t *testing.T) {
			v.Istack().Clear()
			v.Estack().Clear()
			invokeMethod(t, fmt.Sprintf("F%d", i), ne.Script, v, di)
			runAndCheck(t, v, tc.result)
		})
	}
}

func addBoolExprTestFunc(testCases []testCase, b *bytes.Buffer, val bool, cond string) []testCase {
	n := len(testCases)
	b.WriteString(fmt.Sprintf(`
	func F%d_expr() int {
		var cond%d = %s
		if cond%d { return 42 }
		return 17
	}
	func F%d_cond() int {
		if %s { return 42 }
		return 17
	}
	func F%d_else() int {
		if %s { return 42 } else { return 17 }
	}
	`, n, n, cond, n, n, cond, n, cond))

	res := big.NewInt(42)
	if !val {
		res.SetInt64(17)
	}

	return append(testCases, testCase{
		name:   cond,
		result: res,
	})
}

func runBooleanCases(t *testing.T, testCases []testCase, src string) {
	ne, di, err := compiler.CompileWithOptions("file.go", strings.NewReader(src), nil)
	require.NoError(t, err)

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("AsExpression", func(t *testing.T) {
				v := vm.New()
				invokeMethod(t, fmt.Sprintf("F%d_expr", i), ne.Script, v, di)
				runAndCheck(t, v, tc.result)
			})
			t.Run("InCondition", func(t *testing.T) {
				v := vm.New()
				invokeMethod(t, fmt.Sprintf("F%d_cond", i), ne.Script, v, di)
				runAndCheck(t, v, tc.result)
			})
			t.Run("InConditionWithElse", func(t *testing.T) {
				v := vm.New()
				invokeMethod(t, fmt.Sprintf("F%d_else", i), ne.Script, v, di)
				runAndCheck(t, v, tc.result)
			})
		})
	}
}

// TestBooleanExprs enumerates a lot of possible combinations of boolean expressions
// and tests if the result matches to that of Go.
func TestBooleanExprs(t *testing.T) {
	header := `package foo
	var s = "str"
	var v = 9
	`

	srcBuilder := bytes.NewBuffer([]byte(header))

	var testCases []testCase

	trueExpr := []string{"true", "v < 10", "v <= 9", "v > 8", "v >= 9", "v == 9", "v != 8", `s == "str"`}
	falseExpr := []string{"false", "v > 9", "v >= 10", "v < 9", "v <= 8", "v == 8", "v != 9", `s == "a"`}
	t.Run("Single", func(t *testing.T) {
		for _, s := range trueExpr {
			testCases = addBoolExprTestFunc(testCases, srcBuilder, true, s)
		}
		for _, s := range falseExpr {
			testCases = addBoolExprTestFunc(testCases, srcBuilder, false, s)
		}
		runBooleanCases(t, testCases, srcBuilder.String())
	})

	type arg struct {
		val bool
		s   string
	}

	var double []arg
	for _, e := range trueExpr {
		double = append(double, arg{true, e + " || false"})
		double = append(double, arg{true, e + " && true"})
	}
	for _, e := range falseExpr {
		double = append(double, arg{false, e + " && true"})
		double = append(double, arg{false, e + " || false"})
	}

	t.Run("Double", func(t *testing.T) {
		testCases = testCases[:0]
		srcBuilder.Reset()
		srcBuilder.WriteString(header)
		for i := range double {
			testCases = addBoolExprTestFunc(testCases, srcBuilder, double[i].val, double[i].s)
		}
		runBooleanCases(t, testCases, srcBuilder.String())
	})

	var triple []arg
	for _, a1 := range double {
		for _, a2 := range double {
			triple = append(triple, arg{a1.val || a2.val, fmt.Sprintf("(%s) || (%s)", a1.s, a2.s)})
			triple = append(triple, arg{a1.val && a2.val, fmt.Sprintf("(%s) && (%s)", a1.s, a2.s)})
		}
	}

	t.Run("Triple", func(t *testing.T) {
		const step = 350 // empirically found value to make script less than 65536 in size
		for start := 0; start < len(triple); start += step {
			testCases = testCases[:0]
			srcBuilder.Reset()
			srcBuilder.WriteString(header)
			for i := start; i < start+step && i < len(triple); i++ {
				testCases = addBoolExprTestFunc(testCases, srcBuilder, triple[i].val, triple[i].s)
			}
			runBooleanCases(t, testCases, srcBuilder.String())
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
