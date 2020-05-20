package compiler_test

import (
	"math/big"
	"testing"
)

func TestCustomType(t *testing.T) {
	src := `
		package foo

		type bar int
		type specialString string

		func Main() specialString {
			var x bar
			var str specialString
			x = 10
			str = "some short string"
			if x == 10 {
				return str
			}
			return "none"
		}
	`
	eval(t, src, []byte("some short string"))
}

func TestCustomTypeMethods(t *testing.T) {
	src := `package foo
	type bar int
	func (b bar) add(a bar) bar { return a + b }
	func Main() bar {
		var b bar
		b = 10
		return b.add(32)
	}`
	eval(t, src, big.NewInt(42))
}
