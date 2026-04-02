package compiler_test

import (
	"math/big"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/stretchr/testify/require"
)

func TestFuncLiteral(t *testing.T) {
	src := `package foo
	func Main() int {
		inc := func(x int) int { return x + 1 }
		return inc(1) + inc(2)
	}`
	eval(t, src, big.NewInt(5))
}

func TestCallInPlace(t *testing.T) {
	src := `package foo
	var a int = 1
	func Main() int {
		func() {
			a += 10
		}()
		a += 100
		return a
	}`
	eval(t, src, big.NewInt(111))
}

func TestLambdaInDebugInfo(t *testing.T) {
	srcSimple := `package main
		import "github.com/nspcc-dev/neo-go/pkg/interop/runtime"
		func PublicContractMethod() {
			var f = func() {
				runtime.Log("bla")
			}
		
			f()
			f()
		}
	`
	srcFromArray := `package main
		import "github.com/nspcc-dev/neo-go/pkg/interop/runtime"
		func PublicContractMethod() {
			arr := make([]func(), 1)
			arr[0] = func() { runtime.Log("bla") }
			f := arr[0]
			f()
		}
	`
	srcFromAnotherFunc := `package main
		import "github.com/nspcc-dev/neo-go/pkg/interop/runtime"
		func PublicContractMethod() {
			var f = func() func() {
				return func() { runtime.Log("bla") }
			}()
			f()
		}
	`
	for _, src := range []string{srcSimple, srcFromArray, srcFromAnotherFunc} {
		_, di, err := compiler.CompileWithOptions("test.go", strings.NewReader(src), nil)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(di.Methods), 2)
		require.True(t, func() bool {
			for _, methodDebugInfo := range di.Methods {
				if strings.Contains(methodDebugInfo.Name.Name, "lambda") {
					return true
				}
			}
			return false
		}())
	}
}
