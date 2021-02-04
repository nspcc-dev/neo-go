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
			b := 1
			return (b + a) * (b + a)
		}
	}`
	b2, err := compiler.Compile("foo.go", strings.NewReader(src2))
	require.NoError(t, err)
	require.Equal(t, b2, b1)
}
