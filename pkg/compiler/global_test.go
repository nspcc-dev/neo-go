package compiler_test

import (
	"bytes"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/require"
)

func TestUnusedGlobal(t *testing.T) {
	t.Run("simple unused", func(t *testing.T) {
		src := `package foo
				const (
					_ int = iota
					a
				)
				func Main() int {
					return 1
				}`
		prog := eval(t, src, big.NewInt(1))
		require.Equal(t, 2, len(prog)) // PUSH1 + RET
	})
	t.Run("unused with function call inside", func(t *testing.T) {
		t.Run("specification names count matches values count", func(t *testing.T) {
			src := `package foo
				var control int
				var _ = f()
				func Main() int {
					return control
				}
				func f() int {
					control = 1
					return 5
				}`
			eval(t, src, big.NewInt(1))
		})
		t.Run("specification names count differs from values count", func(t *testing.T) {
			src := `package foo
				var control int
				var _, _ = f()
				func Main() int {
					return control
				}
				func f() (int, int) {
					control = 1
					return 5, 6
				}`
			eval(t, src, big.NewInt(1))
		})
		t.Run("used", func(t *testing.T) {
			src := `package foo
				var _, A = f()
				func Main() int {
					return A
				}
				func f() (int, int) {
					return 5, 6
				}`
			eval(t, src, big.NewInt(6))
		})
	})
	t.Run("unused without function call", func(t *testing.T) {
		src := `package foo
				var _ = 1
				var (
					_ = 2 + 3
					_, _ = 3 + 4, 5
				)
				func Main() int {
					return 1
				}`
		prog := eval(t, src, big.NewInt(1))
		require.Equal(t, 2, len(prog)) // PUSH1 + RET
	})
}

func TestChangeGlobal(t *testing.T) {
	src := `package foo
	var a int
	func Main() int {
		setLocal()
		set42()
		setLocal()
		return a
	}
	func set42() { a = 42 }
	func setLocal() { a := 10; _ = a }`

	eval(t, src, big.NewInt(42))
}

func TestMultiDeclaration(t *testing.T) {
	src := `package foo
	var a, b, c int
	func Main() int {
		a = 1
		b = 2
		c = 3
		return a + b + c
	}`
	eval(t, src, big.NewInt(6))
}

func TestCountLocal(t *testing.T) {
	src := `package foo
	func Main() int {
		a, b, c, d := f()
		return a + b + c + d
	}
	func f() (int, int, int, int) {
		return 1, 2, 3, 4
	}`
	eval(t, src, big.NewInt(10))
}

func TestMultiDeclarationLocal(t *testing.T) {
	src := `package foo
	func Main() int {
		var a, b, c int
		a = 1
		b = 2
		c = 3
		return a + b + c
	}`
	eval(t, src, big.NewInt(6))
}

func TestMultiDeclarationLocalCompound(t *testing.T) {
	src := `package foo
	func Main() int {
		var a, b, c []int
		a = append(a, 1)
		b = append(b, 2)
		c = append(c, 3)
		return a[0] + b[0] + c[0]
	}`
	eval(t, src, big.NewInt(6))
}

func TestShadow(t *testing.T) {
	srcTmpl := `package foo
	func Main() int {
		x := 1
		y := 10
		%s
			x += 1  // increase old local
			x := 30 // introduce new local
			y += x  // make sure is means something
		}
		return x+y
	}`

	runCase := func(b string) func(t *testing.T) {
		return func(t *testing.T) {
			src := fmt.Sprintf(srcTmpl, b)
			eval(t, src, big.NewInt(42))
		}
	}

	t.Run("If", runCase("if true {"))
	t.Run("For", runCase("for i := 0; i < 1; i++ {"))
	t.Run("Range", runCase("for range []int{1} {"))
	t.Run("Switch", runCase("switch true {\ncase false: x += 2\ncase true:"))
	t.Run("Block", runCase("{"))
}

func TestArgumentLocal(t *testing.T) {
	srcTmpl := `package foo
	func some(a int) int {
	    if a > 42 {
	        a := 24
			_ = a
	    }
	    return a
	}
	func Main() int {
		return some(%d)
	}`
	t.Run("Override", func(t *testing.T) {
		src := fmt.Sprintf(srcTmpl, 50)
		eval(t, src, big.NewInt(50))
	})
	t.Run("NoOverride", func(t *testing.T) {
		src := fmt.Sprintf(srcTmpl, 40)
		eval(t, src, big.NewInt(40))
	})
}

func TestContractWithNoMain(t *testing.T) {
	src := `package foo
	var someGlobal int = 1
	func Add3(a int) int {
		someLocal := 2
		return someGlobal + someLocal + a
	}`
	b, di, err := compiler.CompileWithOptions("foo.go", strings.NewReader(src), nil)
	require.NoError(t, err)
	v := vm.New()
	invokeMethod(t, "Add3", b.Script, v, di)
	v.Estack().PushVal(39)
	require.NoError(t, v.Run())
	require.Equal(t, 1, v.Estack().Len())
	require.Equal(t, big.NewInt(42), v.PopResult())
}

func TestMultipleFiles(t *testing.T) {
	src := `package foo
	import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/multi"
	func Main() int {
		return multi.Sum()
	}`
	eval(t, src, big.NewInt(42))
}

func TestExportedVariable(t *testing.T) {
	t.Run("Use", func(t *testing.T) {
		src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/multi"
		func Main() int {
			return multi.SomeVar12
		}`
		eval(t, src, big.NewInt(12))
	})
	t.Run("ChangeAndUse", func(t *testing.T) {
		src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/multi"
		func Main() int {
			multi.SomeVar12 = 10
			return multi.Sum()
		}`
		eval(t, src, big.NewInt(40))
	})
	t.Run("PackageAlias", func(t *testing.T) {
		src := `package foo
		import kek "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/multi"
		func Main() int {
			kek.SomeVar12 = 10
			return kek.Sum()
		}`
		eval(t, src, big.NewInt(40))
	})
	t.Run("DifferentName", func(t *testing.T) {
		src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/strange"
		func Main() int {
			normal.NormalVar = 42
			return normal.NormalVar
		}`
		eval(t, src, big.NewInt(42))
	})
	t.Run("MultipleEqualNames", func(t *testing.T) {
		src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/multi"
		var SomeVar12 = 1
		func Main() int {
			SomeVar30 := 3
			sum := SomeVar12 + multi.SomeVar30
			sum += SomeVar30
			sum += multi.SomeVar12
			return sum
		}`
		eval(t, src, big.NewInt(46))
	})
}

func TestExportedConst(t *testing.T) {
	t.Run("with vars", func(t *testing.T) {
		src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/multi"
		func Main() int {
			return multi.SomeConst
		}`
		eval(t, src, big.NewInt(42))
	})
	t.Run("const only", func(t *testing.T) {
		src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/constonly"
		func Main() int {
			return constonly.Answer
		}`
		eval(t, src, big.NewInt(42))
	})
}

func TestMultipleFuncSameName(t *testing.T) {
	t.Run("Simple", func(t *testing.T) {
		src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/multi"
		func Main() int {
			return multi.Sum() + Sum()
		}
		func Sum() int {
			return 11
		}`
		eval(t, src, big.NewInt(53))
	})
	t.Run("WithMethod", func(t *testing.T) {
		src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/foo"
		type Foo struct{}
		func (f Foo) Bar() int { return 11 }
		func Bar() int { return 22 }
		func Main() int {
			var a Foo
			var b foo.Foo
			return a.Bar() + // 11
				foo.Bar() +  // 1
				b.Bar() +    // 8
				Bar()        // 22
		}`
		eval(t, src, big.NewInt(42))
	})
}

func TestConstDontUseSlots(t *testing.T) {
	const count = 256
	buf := bytes.NewBufferString("package foo\n")
	for i := 0; i < count; i++ {
		buf.WriteString(fmt.Sprintf("const n%d = 1\n", i))
	}
	buf.WriteString("func Main() int { sum := 0\n")
	for i := 0; i < count; i++ {
		buf.WriteString(fmt.Sprintf("sum += n%d\n", i))
	}
	buf.WriteString("return sum }")

	src := buf.String()
	eval(t, src, big.NewInt(count))
}

func TestUnderscoreVarsDontUseSlots(t *testing.T) {
	const count = 128
	buf := bytes.NewBufferString("package foo\n")
	for i := 0; i < count; i++ {
		buf.WriteString(fmt.Sprintf("var _, n%d = 1, 1\n", i))
	}
	buf.WriteString("func Main() int { sum := 0\n")
	for i := 0; i < count; i++ {
		buf.WriteString(fmt.Sprintf("sum += n%d\n", i))
	}
	buf.WriteString("return sum }")

	src := buf.String()
	eval(t, src, big.NewInt(count))
}

func TestUnderscoreGlobalVarDontEmitCode(t *testing.T) {
	src := `package foo
		var _ int
		var _ = 1
		var (
			A = 2
			_ = A + 3
			_, B, _ = 4, 5, 6
			_, C, _ = f(A, B)
		)
		var D = 7 // unused but named, so the code is expected
		func Main() int {
			return 1
		}
		func f(a, b int) (int, int, int) {
			return 8, 9, 10
		}`
	eval(t, src, big.NewInt(1), []interface{}{opcode.INITSSLOT, []byte{4}}, // sslot for A, B, C, D
		opcode.PUSH2, opcode.STSFLD0, // store A
		opcode.PUSH5, opcode.STSFLD1, // store B
		opcode.LDSFLD0, opcode.LDSFLD1, opcode.SWAP, []interface{}{opcode.CALL, []byte{10}}, // evaluate f
		opcode.DROP, opcode.STSFLD2, opcode.DROP, // store C
		opcode.PUSH7, opcode.STSFLD3, opcode.RET, // store D
		opcode.PUSH1, opcode.RET, // Main
		[]interface{}{opcode.INITSLOT, []byte{0, 2}}, opcode.PUSH10, opcode.PUSH9, opcode.PUSH8, opcode.RET) // f
}
