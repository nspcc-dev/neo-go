package compiler_test

import (
	"math/big"
	"testing"
)

func TestAddressOfLiteral(t *testing.T) {
	src := `package foo
	type Foo struct { A int	}
	func Main() int {
		f := &Foo{}
		setA(f, 3)
		return f.A
	}
	func setA(s *Foo, a int) { s.A = a }`
	eval(t, src, big.NewInt(3))
}
