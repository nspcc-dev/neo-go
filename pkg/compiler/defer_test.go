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

func TestRecover(t *testing.T) {
	t.Run("Panic", func(t *testing.T) {
		src := `package foo
		var a int
		func Main() int {
			return h() + a
		}
		func h() int {
			defer func() {
				if r := recover(); r != nil {
					a = 3
				} else {
					a = 4
				}
			}()
			a = 1
			panic("msg")
			return a
		}`
		eval(t, src, big.NewInt(3))
	})
	t.Run("NoPanic", func(t *testing.T) {
		src := `package foo
		var a int
		func Main() int {
			return h() + a
		}
		func h() int {
			defer func() {
				if r := recover(); r != nil {
					a = 3
				} else {
					a = 4
				}
			}()
			a = 1
			return a
		}`
		eval(t, src, big.NewInt(5))
	})
	t.Run("PanicInDefer", func(t *testing.T) {
		src := `package foo
		var a int
		func Main() int {
			return h() + a
		}
		func h() int {
			defer func() { a += 2; _ = recover() }()
			defer func() { a *= 3; _ = recover(); panic("again") }()
			a = 1
			panic("msg")
			return a
		}`
		eval(t, src, big.NewInt(5))
	})
}
