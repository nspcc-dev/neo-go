package compiler_test

import (
	"math/big"
	"testing"
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
