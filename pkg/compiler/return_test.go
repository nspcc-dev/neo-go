package compiler_test

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReturnInt64(t *testing.T) {
	src := `package foo
	func Main() int64 {
		return 1
	}`
	eval(t, src, big.NewInt(1))
}

func TestMultipleReturn1(t *testing.T) {
	src := `
		package hello

		func two() (int, int) {
			return 5, 9
		}

		func Main() int {
			a, _ := two()
			return a
		}
	`
	eval(t, src, big.NewInt(5))
}

func TestMultipleReturn2(t *testing.T) {
	src := `
		package hello

		func two() (int, int) {
			return 5, 9
		}

		func Main() int {
			_, b := two()
			return b
		}
	`
	eval(t, src, big.NewInt(9))
}

func TestMultipleReturnUnderscore(t *testing.T) {
	src := `
		package hello
		func f3() (int, int, int) {
			return 5, 6, 7
		}

		func Main() int {
			a, _, c := f3()
			return a+c
		}
	`
	eval(t, src, big.NewInt(12))
}

func TestMultipleReturnWithArg(t *testing.T) {
	src := `
		package hello
		
		func inc2(a int) (int, int) {
			return a+1, a+2 
		}

		func Main() int {
			a, b := 3, 9
			a, b = inc2(a)
			return a+b
		}
	`
	eval(t, src, big.NewInt(9))
}

func TestSingleReturn(t *testing.T) {
	src := `
		package hello

		func inc(k int) int {
			return k+1
		}

		func Main() int {
			a, b := inc(3), inc(4)
			return a+b
		}
	`
	eval(t, src, big.NewInt(9))
}

func TestNamedReturn(t *testing.T) {
	src := `package foo
	func Main() (a int, b int) {
		a = 1
		b = 2
		c := 3
		_ = c
		return %s
	}`

	runCase := func(ret string, result ...interface{}) func(t *testing.T) {
		return func(t *testing.T) {
			src := fmt.Sprintf(src, ret)
			v := vmAndCompile(t, src)
			require.NoError(t, v.Run())
			require.Equal(t, len(result), v.Estack().Len())
			for i := range result {
				assert.EqualValues(t, result[i], v.Estack().Pop().Value())
			}
		}
	}

	t.Run("NormalReturn", runCase("a, b", big.NewInt(1), big.NewInt(2)))
	t.Run("EmptyReturn", runCase("", big.NewInt(1), big.NewInt(2)))
	t.Run("AnotherVariable", runCase("b, c", big.NewInt(2), big.NewInt(3)))
}

func TestTypeAssertReturn(t *testing.T) {
	src := `
		package main

		func foo() interface{} {
			return 5
		}

		func Main() int {
			return foo().(int)
		}
	`
	eval(t, src, big.NewInt(5))
}
