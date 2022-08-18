package compiler_test

import (
	"math/big"
	"testing"
)

func TestGenDeclWithMultiRet(t *testing.T) {
	t.Run("global var decl", func(t *testing.T) {
		src := `package foo
				func Main() int {
					var a, b = f()
					return a + b
				}
				func f() (int, int) {
					return 1, 2
				}`
		eval(t, src, big.NewInt(3))
	})
	t.Run("local var decl", func(t *testing.T) {
		src := `package foo
				var a, b = f()
				func Main() int {
					return a + b
				}
				func f() (int, int) {
					return 1, 2
				}`
		eval(t, src, big.NewInt(3))
	})
}
