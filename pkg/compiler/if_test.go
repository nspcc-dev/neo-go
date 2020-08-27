package compiler_test

import (
	"fmt"
	"math/big"
	"testing"
)

func TestLT(t *testing.T) {
	src := `
		package foo
		func Main() int {
			x := 10
			if x < 100 {
				return 1
			}
			return 0
		}
	`
	eval(t, src, big.NewInt(1))
}

func TestGT(t *testing.T) {
	src := `
		package testcase
		func Main() int {
			x := 10
			if x > 100 {
				return 1
			}
			return 0
		}
	`
	eval(t, src, big.NewInt(0))
}

func TestGTE(t *testing.T) {
	src := `
		package testcase
		func Main() int {
			x := 10
			if x >= 100 {
				return 1
			}
			return 0
		}
	`
	eval(t, src, big.NewInt(0))
}

func TestLAND(t *testing.T) {
	src := `
		package testcase
		func Main() int {
			x := 10
			if x >= 10 && x <= 20 {
				return 1
			}
			return 0
		}
	`
	eval(t, src, big.NewInt(1))
}

func TestLOR(t *testing.T) {
	src := `
		package testcase
		func Main() int {
			x := 10
			if x >= 10 || x <= 20 {
				return 1
			}
			return 0
		}
	`
	eval(t, src, big.NewInt(1))
}

func TestNestedIF(t *testing.T) {
	src := `
		package testcase
		func Main() int {
			x := 10
			if x > 10 {
				if x < 20 {
					return 1
				}
				return 2
			}
			return 0
		}
	`
	eval(t, src, big.NewInt(0))
}

func TestInitIF(t *testing.T) {
	t.Run("Simple", func(t *testing.T) {
		src := `package foo
		func Main() int {
			if a := 42; true {
				return a
			}
			return 0
		}`
		eval(t, src, big.NewInt(42))
	})
	t.Run("Shadow", func(t *testing.T) {
		srcTmpl := `package foo
		func Main() int {
			a := 11
			if a := 42; %v {
				return a
			}
			return a
		}`
		t.Run("True", func(t *testing.T) {
			eval(t, fmt.Sprintf(srcTmpl, true), big.NewInt(42))
		})
		t.Run("False", func(t *testing.T) {
			eval(t, fmt.Sprintf(srcTmpl, false), big.NewInt(11))
		})
	})
}
