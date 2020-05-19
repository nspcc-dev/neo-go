package compiler_test

import (
	"math/big"
	"testing"
)

func TestChangeGlobal(t *testing.T) {
	src := `package foo
	var a int
	func Main() int {
		setLocal()
		set42()
		setLocal()
		return a
	}
	func set42() { a = 42 }
	func setLocal() { a := 10; _ = a }`

	eval(t, src, big.NewInt(42))
}

func TestMultiDeclaration(t *testing.T) {
	src := `package foo
	var a, b, c int
	func Main() int {
		a = 1
		b = 2
		c = 3
		return a + b + c
	}`
	eval(t, src, big.NewInt(6))
}

func TestMultiDeclarationLocal(t *testing.T) {
	src := `package foo
	func Main() int {
		var a, b, c int
		a = 1
		b = 2
		c = 3
		return a + b + c
	}`
	eval(t, src, big.NewInt(6))
}

func TestMultiDeclarationLocalCompound(t *testing.T) {
	src := `package foo
	func Main() int {
		var a, b, c []int
		a = append(a, 1)
		b = append(b, 2)
		c = append(c, 3)
		return a[0] + b[0] + c[0]
	}`
	eval(t, src, big.NewInt(6))
}
