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

func TestReturnValueReceiver(t *testing.T) {
	t.Run("regular", func(t *testing.T) {
		src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/method"

		func Main() int {
			return method.NewX().GetA()
		}`
		eval(t, src, big.NewInt(42))
	})
	t.Run("inline", func(t *testing.T) {
		src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/inline"

		func Main() int {
			return inline.NewT().GetN()
		}`
		eval(t, src, big.NewInt(42))
	})
}

func TestSimpleFunctionCall(t *testing.T) {
	src := `
		package testcase
		func Main() int {
			x := 10
			y := getSomeInteger()
			return x + y
		}

		func getSomeInteger() int {
			x := 10
			return x
		}
	`
	eval(t, src, big.NewInt(20))
}

func TestNotAssignedFunctionCall(t *testing.T) {
	t.Run("Simple", func(t *testing.T) {
		src := `package testcase
		func Main() int {
			getSomeInteger()
			getSomeInteger()
			return 0
		}

		func getSomeInteger() int {
			return 0
		}`
		eval(t, src, big.NewInt(0))
	})
	t.Run("If", func(t *testing.T) {
		src := `package testcase
		func f() bool { return true }
		func Main() int {
			if f() {
				return 42
			}
			return 0
		}`
		eval(t, src, big.NewInt(42))
	})
	t.Run("Switch", func(t *testing.T) {
		src := `package testcase
		func f() bool { return true }
		func Main() int {
			switch true {
			case f():
				return 42
			default:
				return 0
			}
		}`
		eval(t, src, big.NewInt(42))
	})
	t.Run("Builtin", func(t *testing.T) {
		src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/interop/util"
		func Main() int {
			util.FromAddress("NPAsqZkx9WhNd4P72uhZxBhLinSuNkxfB8")
			util.FromAddress("NPAsqZkx9WhNd4P72uhZxBhLinSuNkxfB8")
			return 1
		}`
		eval(t, src, big.NewInt(1))
	})
	t.Run("Lambda", func(t *testing.T) {
		src := `package foo
		func Main() int {
			f := func() (int, int) { return 1, 2 }
			f()
			f()
			return 42
		}`
		eval(t, src, big.NewInt(42))
	})
	t.Run("VarDecl", func(t *testing.T) {
		src := `package foo
		func foo() []int { return []int{1} }
		func Main() int {
			var x = foo()
			return len(x)
		}`
		eval(t, src, big.NewInt(1))
	})
}

func TestMultipleFunctionCalls(t *testing.T) {
	src := `
		package testcase
		func Main() int {
			x := 10
			y := getSomeInteger()
			return x + y
		}

		func getSomeInteger() int {
			x := 10
			y := getSomeOtherInt()
			return x + y
		}

		func getSomeOtherInt() int {
			x := 8
			return x
		}
	`
	eval(t, src, big.NewInt(28))
}

func TestFunctionCallWithArgs(t *testing.T) {
	src := `
		package testcase
		func Main() int {
			x := 10
			y := getSomeInteger(x)
			return y
		}

		func getSomeInteger(x int) int {
			y := 8
			return x + y
		}
	`
	eval(t, src, big.NewInt(18))
}

func TestFunctionCallWithInterfaceType(t *testing.T) {
	src := `
		package testcase
		func Main() interface{} {
			x := getSomeInteger(10)
			return x
		}

		func getSomeInteger(x interface{}) interface{} {
			return x
		}
	`
	eval(t, src, big.NewInt(10))
}

func TestFunctionCallWithAnyKeywordType(t *testing.T) {
	src := `
		package testcase
		func Main() any {
			x := getSomeInteger(10)
			return x
		}

		func getSomeInteger(x any) any {
			return x
		}
	`
	eval(t, src, big.NewInt(10))
}

func TestFunctionCallMultiArg(t *testing.T) {
	src := `
		package testcase
		func Main() int {
			x := addIntegers(2, 4)
			return x
		}

		func addIntegers(x int, y int) int {
			return x + y
		}
	`
	eval(t, src, big.NewInt(6))
}

func TestFunctionWithVoidReturn(t *testing.T) {
	src := `
		package testcase
		func Main() int {
			x := 2
			getSomeInteger()
			y := 4
			return x + y
		}

		func getSomeInteger() { %s }
	`
	t.Run("EmptyBody", func(t *testing.T) {
		src := fmt.Sprintf(src, "")
		eval(t, src, big.NewInt(6))
	})
	t.Run("SingleReturn", func(t *testing.T) {
		src := fmt.Sprintf(src, "return")
		eval(t, src, big.NewInt(6))
	})
}

func TestFunctionWithVoidReturnBranch(t *testing.T) {
	src := `
		package testcase
		func Main() int {
			x := %t
			f(x)
			return 2
		}

		func f(x bool) {
			if x {
				return
			}
		}
	`
	t.Run("ReturnBranch", func(t *testing.T) {
		src := fmt.Sprintf(src, true)
		eval(t, src, big.NewInt(2))
	})
	t.Run("NoReturn", func(t *testing.T) {
		src := fmt.Sprintf(src, false)
		eval(t, src, big.NewInt(2))
	})
}

func TestFunctionWithMultipleArgumentNames(t *testing.T) {
	src := `package foo
	func Main() int {
		return add(1, 2)
	}
	func add(a, b int) int {
		return a + b
	}`
	eval(t, src, big.NewInt(3))
}

func TestLocalsCount(t *testing.T) {
	src := `package foo
	func f(a, b, c int) int {
		sum := a
		for i := 0; i < c; i++ {
			sum += b
		}
		return sum
	}
	func Main() int {
		return f(1, 2, 3)
	}`
	eval(t, src, big.NewInt(7))
}

func TestVariadic(t *testing.T) {
	srcTmpl := `package foo
	func someFunc(a int, b ...int) int {
		sum := a
		for i := range b {
			sum = sum - b[i]
		}
		return sum
	}
	func Main() int {
		%s
		return someFunc(10, %s)
	}`
	t.Run("Elements", func(t *testing.T) {
		src := fmt.Sprintf(srcTmpl, "", "1, 2, 3")
		eval(t, src, big.NewInt(4))
	})
	t.Run("Slice", func(t *testing.T) {
		src := fmt.Sprintf(srcTmpl, "a := []int{1, 2, 3}", "a...")
		eval(t, src, big.NewInt(4))
	})
	t.Run("Literal", func(t *testing.T) {
		src := fmt.Sprintf(srcTmpl, "", "[]int{1, 2, 3}...")
		eval(t, src, big.NewInt(4))
	})
}

func TestVariadicMethod(t *testing.T) {
	src := `package foo
	type myInt int
	func (x myInt) someFunc(a int, b ...int) int {
		sum := int(x) + a
		for i := range b {
			sum = sum - b[i] 
		}
		return sum
	}
	func Main() int {
		x := myInt(38)
		return x.someFunc(10, 1, 2, 3)
	}`
	eval(t, src, big.NewInt(42))
}

func TestJumpOptimize(t *testing.T) {
	src := `package foo
	func init() {
		if true {} else {}
		var a int
		_ = a
	}
	func _deploy(_ any, upd bool) {
		if true {} else {}
		t := upd
		_ = t
	}
	func Get1() int { return 1 }
	func Get2() int { Get1(); Get1(); Get1(); Get1(); return Get1() }
	func Get3() int { return Get2() }
	func Main() int {
		return Get3()
	}`
	b, di, err := compiler.CompileWithOptions("file.go", strings.NewReader(src), nil)
	require.NoError(t, err)
	require.Equal(t, 6, len(di.Methods))
	for _, mi := range di.Methods {
		// only _deploy and init have locals here
		if mi.Name.Name == "_deploy" || mi.Name.Name == "init" {
			require.Equal(t, b.Script[mi.Range.Start], byte(opcode.INITSLOT))
		}
		require.Equal(t, b.Script[mi.Range.End], byte(opcode.RET))
	}
}

func TestFunctionUnusedParameters(t *testing.T) {
	src := `package foo
	func add13(a int, _ int, _1 int, _ int) int {
		return a + _1
	}
	func Main() int {
		return add13(1, 10, 100, 1000)
	}`
	eval(t, src, big.NewInt(101))
}

func TestUnusedFunctions(t *testing.T) {
	t.Run("only variable", func(t *testing.T) {
		src := `package foo
		import "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/nestedcall"
		func Main() int {
			return nestedcall.X
		}`

		b, err := compiler.Compile("foo.go", strings.NewReader(src))
		require.NoError(t, err)
		require.Equal(t, 3, len(b)) // PUSHINT8 (42) + RET
		eval(t, src, big.NewInt(42))
	})
	t.Run("imported function", func(t *testing.T) {
		// Check that import map is set correctly during package traversal.
		src := `package foo
		import inner "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/nestedcall"
		func Main() int {
			return inner.N()
		}`

		_, err := compiler.Compile("foo.go", strings.NewReader(src))
		require.NoError(t, err)
		eval(t, src, big.NewInt(65))
	})
	t.Run("method inside of an imported package", func(t *testing.T) {
		// Check that import map is set correctly during package traversal.
		src := `package foo
		import inner "github.com/nspcc-dev/neo-go/pkg/compiler/testdata/nestedcall"
		func Main() int {
			var t inner.Token
			return t.Method()
		}`

		_, err := compiler.Compile("foo.go", strings.NewReader(src))
		require.NoError(t, err)
		eval(t, src, big.NewInt(2231))
	})
}

func TestUnnamedMethodReceiver(t *testing.T) {
	src := `package foo
	type CustomInt int
	func Main() int {
		var i CustomInt
		i = 5
		return i.Do(2)
	}
	func (CustomInt) Do(arg int) int {
		return arg
	}`
	eval(t, src, big.NewInt(2))
}
