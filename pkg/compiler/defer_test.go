package compiler_test

import (
	"math/big"
	"testing"
)

func TestDefer(t *testing.T) {
	t.Run("Simple", func(t *testing.T) {
		src := `package main
		var a int
		func Main() int {
			return h() + a
		}
		func h() int {
			defer f()
			return 1
		}
		func f() { a += 2 }`
		eval(t, src, big.NewInt(3))
	})
	t.Run("ValueUnchanged", func(t *testing.T) {
		src := `package main
		var a int
		func Main() int {
			defer f()
			a = 3
			return a
		}
		func f() { a += 2 }`
		eval(t, src, big.NewInt(3))
	})
	t.Run("Function", func(t *testing.T) {
		src := `package main
		var a int
		func Main() int {
			return h() + a
		}
		func h() int {
			defer f()
			a = 3
			return g()
		}
		func g() int {
			a++
			return a
		}
		func f() { a += 2 }`
		eval(t, src, big.NewInt(10))
	})
	t.Run("MultipleDefers", func(t *testing.T) {
		src := `package main
		var a int
		func Main() int {
			return h() + a
		}
		func h() int {
			defer f()
			defer g()
			a = 3
			return a
		}
		func g() { a *= 2 }
		func f() { a += 2 }`
		eval(t, src, big.NewInt(11))
	})
	t.Run("FunctionLiteral", func(t *testing.T) {
		src := `package main
		var a int
		func Main() int {
			return h() + a
		}
		func h() int {
			defer func() {
				a = 10
			}()
			a = 3
			return a
		}`
		eval(t, src, big.NewInt(13))
	})
}
