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
