package vm_test

import (
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
