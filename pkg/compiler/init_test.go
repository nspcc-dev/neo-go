package compiler_test

import (
	"math/big"
	"testing"
)

func TestInit(t *testing.T) {
	t.Run("Simple", func(t *testing.T) {
		src := `package foo
		var a int
		func init() {
			a = 42
		}
		func Main() int {
			return a
		}`
		eval(t, src, big.NewInt(42))
	})
	t.Run("Multi", func(t *testing.T) {
		src := `package foo
		var m = map[int]int{}
		var a = 2
		func init() {
			m[1] = 11
		}
		func init() {
			a = 1
			m[3] = 30
		}
		func Main() int {
			return m[1] + m[3] + a
		}`
		eval(t, src, big.NewInt(42))
	})
	t.Run("WithCall", func(t *testing.T) {
		src := `package foo
		var m = map[int]int{}
		func init() {
			initMap(m)
		}
		func initMap(m map[int]int) {
			m[11] = 42
		}
		func Main() int {
			return m[11]
		}`
		eval(t, src, big.NewInt(42))
	})
	t.Run("InvalidSignature", func(t *testing.T) {
		src := `package foo
		type Foo int
		var a int
		func (f Foo) init() {
			a = 2
		}
		func Main() int {
			return a
		}`
		eval(t, src, big.NewInt(0))
	})
}
