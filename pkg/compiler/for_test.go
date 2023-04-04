package compiler_test

import (
	"bytes"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func TestEntryPointWithMethod(t *testing.T) {
	src := `
		package foo

		func Main(op string) int {
			if op == "a" {
				return 1
			}
			return 0
		}
	`
	evalWithArgs(t, src, []byte("a"), nil, big.NewInt(1))
}

func TestEntryPointWithArgs(t *testing.T) {
	src := `
		package foo

		func Main(args []any) int {
			return 2 + args[1].(int)
		}
	`
	args := []stackitem.Item{stackitem.NewBigInteger(big.NewInt(0)), stackitem.NewBigInteger(big.NewInt(1))}
	evalWithArgs(t, src, nil, args, big.NewInt(3))
}

func TestEntryPointWithMethodAndArgs(t *testing.T) {
	src := `
		package foo

		func Main(method string, args []any) int {
			if method == "foobar" {
				return 2 + args[1].(int)
			}
			return 0
		}
	`
	args := []stackitem.Item{stackitem.NewBigInteger(big.NewInt(0)), stackitem.NewBigInteger(big.NewInt(1))}
	evalWithArgs(t, src, []byte("foobar"), args, big.NewInt(3))
}

func TestArrayFieldInStruct(t *testing.T) {
	src := `
		package foo

		type Bar struct {
			arr []int
		}

		func Main() int {
			b := Bar{
				arr: []int{0, 1, 2},
			}
			x := b.arr[2]
			return x + 2
		}
	`
	eval(t, src, big.NewInt(4))
}

func TestArrayItemGetIndexBinaryExpr(t *testing.T) {
	src := `
		package foo
		func Main() int {
			x := 1
			y := []int{0, 1, 2}
			return y[x + 1]
		}
	`
	eval(t, src, big.NewInt(2))
}

func TestArrayItemGetIndexIdent(t *testing.T) {
	src := `
		package foo
		func Main() int {
			x := 1
			y := []int{0, 1, 2}
			return y[x]
		}
	`
	eval(t, src, big.NewInt(1))
}

func TestArrayItemBinExpr(t *testing.T) {
	src := `
		package foo
		func Main() int {
			x := []int{0, 1, 2}
			return x[1] + 10
		}
	`
	eval(t, src, big.NewInt(11))
}

func TestArrayItemReturn(t *testing.T) {
	src := `
		package foo
		func Main() int {
			arr := []int{0, 1, 2}
			return arr[1]
		}
	`
	eval(t, src, big.NewInt(1))
}

func TestArrayItemAssign(t *testing.T) {
	src := `
		package foo
		func Main() int {
			arr := []int{1, 2, 3}
			y := arr[0]
			return y
		}
	`
	eval(t, src, big.NewInt(1))
}

func TestStringArray(t *testing.T) {
	src := `
		package foo
		func Main() []string {
			x := []string{"foo", "bar", "foobar"}
			return x
		}
	`
	eval(t, src, []stackitem.Item{
		stackitem.NewByteArray([]byte("foo")),
		stackitem.NewByteArray([]byte("bar")),
		stackitem.NewByteArray([]byte("foobar")),
	})
}

func TestIntArray(t *testing.T) {
	src := `
		package foo
		func Main() []int {
			arr := []int{1, 2, 3}
			return arr
		}
	`
	eval(t, src, []stackitem.Item{
		stackitem.NewBigInteger(big.NewInt(1)),
		stackitem.NewBigInteger(big.NewInt(2)),
		stackitem.NewBigInteger(big.NewInt(3)),
	})
}

func TestArrayLen(t *testing.T) {
	src := `
		package foo
		func Main() int {
			arr := []int{0, 1, 2}
			return len(arr)
		}
	`
	eval(t, src, big.NewInt(3))
}

func TestStringLen(t *testing.T) {
	src := `
		package foo
		func Main() int {
			str := "this is medium sized string"
			return len(str)
		}
	`
	eval(t, src, big.NewInt(27))
}

func TestByteArrayLen(t *testing.T) {
	src := `
		package foo

		func Main() int {
			b := []byte{0x00, 0x01, 0x2}
			return len(b)
		}
	`
	eval(t, src, big.NewInt(3))
}

func TestSimpleString(t *testing.T) {
	src := `
		package foo
		func Main() string {
			x := "NEO"
			return x
		}
	`
	eval(t, src, stackitem.NewByteArray([]byte("NEO")).Value())
}

func TestBoolAssign(t *testing.T) {
	src := `
		package foo
		func Main() bool {
			x := true
			return x
		}
	`
	eval(t, src, true)
}

func TestBoolCompare(t *testing.T) {
	src := `
		package foo
		func Main() int {
			x := true
			if x {
				return 10
			}
			return 0
		}
	`
	eval(t, src, big.NewInt(10))
}

func TestBoolCompareVerbose(t *testing.T) {
	src := `
		package foo
		func Main() int {
			x := true
			if x == true {
				return 10
			}
			return 0
		}
	`
	eval(t, src, big.NewInt(10))
}

func TestUnaryExpr(t *testing.T) {
	src := `
		package foo
		func Main() bool {
			x := false
			return !x
		}
	`
	eval(t, src, true)
}

func TestIfUnaryInvertPass(t *testing.T) {
	src := `
		package foo
		func Main() int {
			x := false
			if !x { 
				return 10
			}
			return 0
		}
	`
	eval(t, src, big.NewInt(10))
}

func TestIfUnaryInvert(t *testing.T) {
	src := `
		package foo
		func Main() int {
			x := true
			if !x { 
				return 10
			}
			return 0
		}
	`
	eval(t, src, big.NewInt(0))
}

func TestAppendByte(t *testing.T) {
	src := `
	package foo
		func Main() []byte {
			arr := []byte{0x00, 0x01, 0x02}
			arr = append(arr, 0x03)
			arr = append(arr, 0x04)
			arr = append(arr, 0x05)
			arr = append(arr, 0x06)
			return arr
		}
	`
	eval(t, src, []uint8{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06})
}

func TestAppendByteToEmpty(t *testing.T) {
	src := `
	package foo
	func Main() []byte {
		out := []byte{}
		out = append(out, 1)
		out = append(out, 2)
		return out
	}`
	eval(t, src, []byte{1, 2})
}

func TestAppendString(t *testing.T) {
	src := `
	package foo
		func Main() string {
			arr := []string{"a", "b", "c"}
			arr = append(arr, "d")
			return arr[3]
		}
	`
	eval(t, src, stackitem.NewByteArray([]byte("d")).Value())
}

func TestAppendInt(t *testing.T) {
	src := `
	package foo
		func Main() int {
			arr := []int{0, 1, 2}
			arr = append(arr, 3)
			return arr[3]
		}
	`
	eval(t, src, big.NewInt(3))
}

func TestClassicForLoop(t *testing.T) {
	src := `
	package foo
		func Main() int {
			x := 0
			for i := 0; i < 10; i++ {
				x = i
			}
			return x
		}
	`
	eval(t, src, big.NewInt(9))
}

func TestInc(t *testing.T) {
	src := `
	package foo
		func Main() int {
			x := 0
			x++
			return x
		}
	`

	eval(t, src, big.NewInt(1))
}

func TestDec(t *testing.T) {
	src := `
	package foo
		func Main() int {
			x := 2
			x--
			return x
		}
	`

	eval(t, src, big.NewInt(1))
}

var forLoopTestCases = []testCase{
	{
		"empty for loop",
		`func F%d() int {
			x := 0
			for {
				x++
				if x == 2 { break }
			}
			return x
		}
		`,
		big.NewInt(2),
	}, {
		"big iteration count",
		`func F%d() int {
			x := 0
			for i := 0; i < 100000; i++ {
				x = i
			}
			return x
		}
		`,
		big.NewInt(99999),
	},
	{
		"no init",
		`func F%d() int {
			i := 0
			for ; i < 10; i++ {
			}
			return i
		}
	`,
		big.NewInt(10),
	},
	{
		"no post",
		`func F%d() int {
			i := 0
			for i < 10 {
				i++
			}
			return i
		}
		`,
		big.NewInt(10),
	},
	{
		"range",
		`func F%d() int {
			sum := 0
			arr := []int{1, 2, 3}
			for i := range arr {
				sum += arr[i] 
			}
			return sum
		}
		`,
		big.NewInt(6),
	},
	{
		"range, global index",
		`func F%d() int {
			sum := 0
			i := 0
			arr := []int{1, 2, 3}
			for i = range arr {
				sum += arr[i]
			}
			return sum + i
		}
		`,
		big.NewInt(8),
	},
	{
		"range, change variable",
		`func F%d() int {
			sum := 0
			arr := []int{1, 2, 3}
			for i := range arr {
				sum += arr[i]
				i++
				sum += i
			}
			return sum
		}
		`,
		big.NewInt(12),
	},
	{
		"break",
		`func F%d() int {
			var i int
			for i < 10 {
				i++
				if i == 5 {
					break
				}
			}
			return i
		}
		`,
		big.NewInt(5),
	},
	{
		"break label",
		`func F%d() int {
			var i int
			loop:
			for i < 10 {
				i++
				if i == 5 {
					break loop
				}
			}
			return i
		}
		`,
		big.NewInt(5),
	},
	{
		"nested break",
		`func F%d() int {
			var i int
			for i < 10 {
				i++
				for j := 0; j < 2; j++ {
					i++
					if i == 5 {
						break
					}
				}
			}
			return i
		}
		`,
		big.NewInt(11),
	},
	{
		"nested break label",
		`func F%d() int {
			var i int
			loop:
			for i < 10 {
				i++
				for j := 0; j < 2; j++ {
					if i == 5 {
						break loop
					}
					i++
				}
			}
			return i
		}
		`,
		big.NewInt(5),
	},
	{
		"continue",
		`func F%d() int {
			var i, j int
			for i < 10 {
				i++
				if i >= 5 {
					continue
				}
				j++
			}
			return j
		}
		`,
		big.NewInt(4),
	},
	{
		"continue label",
		`func F%d() int {
			var i, j int
			loop:
			for i < 10 {
				i++
				if i >= 5 {
					continue loop
				}
				j++
			}
			return j
		}
		`,
		big.NewInt(4),
	},
	{
		"nested continue",
		`func F%d() int {
			var i, k int
			for i < 10 {
				i++
				for j := 0; j < 3; j++ {
					if j >= 2 {
						continue
					}
					k++
				}
			}
			return k
		}
		`,
		big.NewInt(20),
	},
	{
		"nested continue label",
		`func F%d() int {
			var i int
			loop:
			for ; i < 10; i += 10 {
				i++
				for j := 0; j < 4; j++ {
					if i == 5 {
						continue loop
					}
					i++
				}
			}
			return i
		}
		`,
		big.NewInt(15),
	},
	{
		"range break",
		`func F%d() int {
			var i int
			arr := []int{1, 2, 3}
			for i = range arr {
				if arr[i] == 2 {
					break
				}
			}
			return i
		}
		`,
		big.NewInt(1),
	},
	{
		"range nested break",
		`func F%d() int {
			k := 5
			arr := []int{1, 2, 3}
			urr := []int{4, 5, 6, 7}
			loop:
			for range arr {
				k++
				for j := range urr {
					k++
					if j == 3 {
						break loop
					}
				}
			}
			return k
		}
		`,
		big.NewInt(10),
	},
	{
		"range continue",
		`func F%d() int {
			i := 6
			arr := []int{1, 2, 3}
			for j := range arr {
				if arr[j] < 2 {
					continue
				}
				i++
			}
			return i
		}
		`,
		big.NewInt(8),
	},
	{
		"range, no variable",
		`func F%d() int {
			sum := 0
			arr := []int{1, 2, 3}
			for range arr {
				sum += 1
			}
			return sum
		}
		`,
		big.NewInt(3),
	},
	{
		"range value",
		`func f(a int) int { return a }
		func F%d() int {
			var sum int
			arr := []int{1, 9, 4}
			for _, v := range arr {
				sum += f(v)
			}
			return sum
		}
		`,
		big.NewInt(14),
	},
	{
		"range, map",
		`func F%d() int {
			m := map[int]int{
				1: 13,
				11: 17,
			}
			var sum int
			for i, v := range m {
				sum += i
				sum += v
			}
			return sum
		}
		`,
		big.NewInt(42),
	},
	{
		"range, type conversion",
		`type intArr []int
		func F%d() int {
			a := []int{1, 2, 3}
			s := 0
			for _, v := range intArr(a) {
				s += v
			}
			return s
		}
		`,
		big.NewInt(6),
	},
	{
		"shadow range key",
		`func F%d() int {
			i := 10
			ints := []int{1, 2, 3, 4, 5}
			for i := range ints {
				_ = i
			}
			return i
		}
		`,
		big.NewInt(10),
	},
	{
		"shadow range value",
		`func F%d() int {
			i := 10
			ints := []int{1, 2, 3, 4, 5}
			for _, i := range ints {
				_ = i
			}
			return i
		}
		`,
		big.NewInt(10),
	},
}

func TestForLoop(t *testing.T) {
	srcBuilder := bytes.NewBuffer([]byte("package testcase\n"))
	for i, tc := range forLoopTestCases {
		srcBuilder.WriteString(fmt.Sprintf(tc.src, i))
	}

	ne, di, err := compiler.CompileWithOptions("file.go", strings.NewReader(srcBuilder.String()), nil)
	require.NoError(t, err)

	for i, tc := range forLoopTestCases {
		v := vm.New()
		t.Run(tc.name, func(t *testing.T) {
			v.Reset(trigger.Application)
			invokeMethod(t, fmt.Sprintf("F%d", i), ne.Script, v, di)
			runAndCheck(t, v, tc.result)
		})
	}
}

func TestForLoopComplexConditions(t *testing.T) {
	forCondTestCases := []struct {
		Name   string
		Cond   string
		Assign string
		Result int64
	}{
		{Cond: "i < 3 && j < 2", Result: 2},
		{Cond: "i < 3 || j < 2", Result: 3},
		{Cond: "i < 3 && (j < 2 || i < 1)", Result: 2},
		{Cond: "i < 3 && (j < 2 && i < 1)", Result: 1},
		{Cond: "(i < 1 || j < 3) && (i < 3 || j < 1)", Result: 3},
		{Cond: "(i < 2 && j < 4) || (i < 4 && j < 2)", Result: 2},
		{Cond: "ok", Assign: "ok = i < 3 && j < 2", Result: 2},
		{Cond: "ok", Assign: "ok = i < 3 || j < 2", Result: 3},
		{Cond: "ok", Assign: "ok = i < 3 && (j < 2 || i < 1)", Result: 2},
		{Cond: "ok", Assign: "ok = i < 3 && (j < 2 && i < 1)", Result: 1},
		{Cond: "ok", Assign: "ok = (i < 1 || j < 3) && (i < 3 || j < 1)", Result: 3},
		{Cond: "ok", Assign: "ok = (i < 2 && j < 4) || (i < 4 && j < 2)", Result: 2},
	}

	tmpl := `func F%d() int {
		var ok bool
		_ = ok
		i := 0
		j := 0
		%s
		for %s {
			i++
			j++
			%s
		}
		return i
	}
	`
	srcBuilder := bytes.NewBufferString("package foo\n")
	for i, tc := range forCondTestCases {
		srcBuilder.WriteString(fmt.Sprintf(tmpl, i, tc.Assign, tc.Cond, tc.Assign))
	}

	ne, di, err := compiler.CompileWithOptions("file.go", strings.NewReader(srcBuilder.String()), nil)
	require.NoError(t, err)

	for i, tc := range forCondTestCases {
		v := vm.New()
		name := tc.Cond
		if tc.Assign != "" {
			name = tc.Assign
		}
		t.Run(name, func(t *testing.T) {
			v.Reset(trigger.Application)
			invokeMethod(t, fmt.Sprintf("F%d", i), ne.Script, v, di)
			runAndCheck(t, v, big.NewInt(tc.Result))
		})
	}
}
