package compiler_test

import (
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/stretchr/testify/require"
)

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
	b, di, err := compiler.CompileWithDebugInfo(strings.NewReader(src))
	require.NoError(t, err)
	v := vm.New()
	invokeMethod(t, "Add3", b, v, di)
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
