package compiler_test

import (
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/require"
)

func checkCallCount(t *testing.T, src string, expectedCall, expectedInitSlot int) {
	v := vmAndCompile(t, src)
	ctx := v.Context()
	actualCall := 0
	actualInitSlot := 0

	for op, _, err := ctx.Next(); ; op, _, err = ctx.Next() {
		require.NoError(t, err)
		switch op {
		case opcode.CALL, opcode.CALLL:
			actualCall++
		case opcode.INITSLOT:
			actualInitSlot++
		}
		if ctx.IP() == ctx.LenInstr() {
			break
		}
	}
	require.Equal(t, expectedCall, actualCall)
	require.Equal(t, expectedInitSlot, actualInitSlot)
}

func TestInline(t *testing.T) {
	srcTmpl := `package foo
	import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/inline"
	// local alias
	func sum(a, b int) int {
		return 42
	}
	var Num = 1
	func Main() int {
		%s
	}`
	t.Run("no return", func(t *testing.T) {
		src := fmt.Sprintf(srcTmpl, `inline.NoArgsNoReturn()
			return 1`)
		checkCallCount(t, src, 0, 1)
		eval(t, src, big.NewInt(1))
	})
	t.Run("has return, dropped", func(t *testing.T) {
		src := fmt.Sprintf(srcTmpl, `inline.NoArgsReturn1()
			return 2`)
		checkCallCount(t, src, 0, 1)
		eval(t, src, big.NewInt(2))
	})
	t.Run("drop twice", func(t *testing.T) {
		src := fmt.Sprintf(srcTmpl, `inline.DropInsideInline()
			return 42`)
		checkCallCount(t, src, 0, 1)
		eval(t, src, big.NewInt(42))
	})
	t.Run("no args return 1", func(t *testing.T) {
		src := fmt.Sprintf(srcTmpl, `return inline.NoArgsReturn1()`)
		checkCallCount(t, src, 0, 1)
		eval(t, src, big.NewInt(1))
	})
	t.Run("sum", func(t *testing.T) {
		src := fmt.Sprintf(srcTmpl, `return inline.Sum(1, 2)`)
		checkCallCount(t, src, 0, 1)
		eval(t, src, big.NewInt(3))
	})
	t.Run("sum squared (nested inline)", func(t *testing.T) {
		src := fmt.Sprintf(srcTmpl, `return inline.SumSquared(1, 2)`)
		checkCallCount(t, src, 0, 1)
		eval(t, src, big.NewInt(9))
	})
	t.Run("inline function in inline function parameter", func(t *testing.T) {
		src := fmt.Sprintf(srcTmpl, `return inline.Sum(inline.SumSquared(1, 2), inline.Sum(3, 4))`)
		checkCallCount(t, src, 0, 1)
		eval(t, src, big.NewInt(9+3+4))
	})
	t.Run("global name clash", func(t *testing.T) {
		src := fmt.Sprintf(srcTmpl, `return inline.GetSumSameName()`)
		checkCallCount(t, src, 0, 1)
		eval(t, src, big.NewInt(42))
	})
	t.Run("local name clash", func(t *testing.T) {
		src := fmt.Sprintf(srcTmpl, `return inline.Sum(inline.SumSquared(1, 2), sum(3, 4))`)
		checkCallCount(t, src, 1, 2)
		eval(t, src, big.NewInt(51))
	})
	t.Run("var args, empty", func(t *testing.T) {
		src := fmt.Sprintf(srcTmpl, `return inline.VarSum(11)`)
		checkCallCount(t, src, 0, 1)
		eval(t, src, big.NewInt(11))
	})
	t.Run("var args, direct", func(t *testing.T) {
		src := fmt.Sprintf(srcTmpl, `return inline.VarSum(11, 14, 17)`)
		checkCallCount(t, src, 0, 1)
		eval(t, src, big.NewInt(42))
	})
	t.Run("var args, array", func(t *testing.T) {
		src := fmt.Sprintf(srcTmpl, `arr := []int{14, 17} 
			return inline.VarSum(11, arr...)`)
		checkCallCount(t, src, 0, 1)
		eval(t, src, big.NewInt(42))
	})
	t.Run("globals", func(t *testing.T) {
		src := fmt.Sprintf(srcTmpl, `return inline.Concat(Num)`)
		checkCallCount(t, src, 0, 1)
		eval(t, src, big.NewInt(221))
	})
}

func TestInlineInLoop(t *testing.T) {
	t.Run("simple", func(t *testing.T) {
		src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/interop/storage"
		import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/inline"
		func Main() int {
			sum := 0
			values := []int{10, 11}
			for _, v := range values {
				_ = v // use 'v'
				storage.GetContext() // push something on stack to check it's dropped
				sum += inline.VarSum(1, 2, 3, 4)
			}
			return sum
		}`
		eval(t, src, big.NewInt(20))
	})
	t.Run("inlined argument", func(t *testing.T) {
		src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/interop/runtime"
		import "github.com/nspcc-dev/neo-go/pkg/interop/storage"
		import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/inline"
		func Main() int {
			sum := 0
			values := []int{10, 11}
			for _, v := range values {
				_  = v // use 'v'
				storage.GetContext() // push something on stack to check it's dropped
				sum += inline.VarSum(1, 2, 3, runtime.GetTime()) // runtime.GetTime always returns 4 in these tests
			}
			return sum
		}`
		eval(t, src, big.NewInt(20))
	})
	t.Run("check clean stack on return", func(t *testing.T) {
		src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/interop/storage"
		import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/inline"
		func Main() int {
			values := []int{10, 11, 12}
			for _, v := range values {
				storage.GetContext() // push something on stack to check it's dropped
				if v == 11 {
					return inline.VarSum(2, 20, 200)
				}
			}
			return 0
		}`
		eval(t, src, big.NewInt(222))
	})
}

func TestInlineInSwitch(t *testing.T) {
	src := `package foo
	import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/inline"
	func Main() int {
		switch inline.VarSum(1, 2) {
		case inline.VarSum(3, 1):
			return 10
		case inline.VarSum(4, -1):
			return 11
		default:
			return 12
		}
	}`
	eval(t, src, big.NewInt(11))
}

func TestInlineGlobalVariable(t *testing.T) {
	t.Run("simple", func(t *testing.T) {
		src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/inline"
		var a = inline.Sum(1, 2)
		func Main() int {
			return a
		}`
		eval(t, src, big.NewInt(3))
	})
	t.Run("complex", func(t *testing.T) {
		src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/inline"
		var a = inline.Sum(3, 4)
		var b = inline.SumSquared(1, 2) 
		var c = a + b
		func init() {
			c--
		}
		func Main() int {
			return c
		}`
		eval(t, src, big.NewInt(15))
	})
}

func TestInlineConversion(t *testing.T) {
	src1 := `package foo
	import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/inline"
	var _ = inline.A
	func Main() int {
		a := 2
		return inline.SumSquared(1, a)
	}`
	b1, err := compiler.Compile("foo.go", strings.NewReader(src1))
	require.NoError(t, err)

	src2 := `package foo
	import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/inline"
	var _ = inline.A
	func Main() int {
		a := 2
		{
			return (1 + a) * (1 + a)
		}
	}`
	b2, err := compiler.Compile("foo.go", strings.NewReader(src2))
	require.NoError(t, err)
	require.Equal(t, b2, b1)
}

func TestInlineConversionQualified(t *testing.T) {
	src1 := `package foo
	import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/inline"
	var A = 1
	func Main() int {
		return inline.Concat(A)
	}`
	b1, err := compiler.Compile("foo.go", strings.NewReader(src1))
	require.NoError(t, err)

	src2 := `package foo
	import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/inline"
	import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/inline/b"
	var A = 1
	func Main() int {
		return A * 100 + b.A * 10 + inline.A
	}`
	b2, err := compiler.Compile("foo.go", strings.NewReader(src2))
	require.NoError(t, err)
	require.Equal(t, b2, b1)
}
