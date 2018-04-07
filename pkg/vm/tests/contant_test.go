package vm_test

import (
	"math/big"
	"testing"
)

func TestBasicConstant(t *testing.T) {
	src := `
		package foo

		const x = 10

		func Main() int {
			return x + 2
		}
	`
	eval(t, src, big.NewInt(12))
}

func TestShortHandMultiConst(t *testing.T) {
	src := `
		package foo

		const (
			z = 3
			y = 2
			x = 1
		)

		// should load al 3 constants in the Main.
		func Main() int {
			return x + z + y
		}
	`
	eval(t, src, big.NewInt(6))
}

func TestGlobalsWithFunctionParams(t *testing.T) {
	src := `
		package foobar

		const (
			// complex he o_O
			bar = "FOO" 
			foo = "BAR"
		)

		func something(x int) string {
			if x > 10 {
				return bar
			}
			return foo
		}

		func Main() string {
			trigger := 100
			x := something(trigger)
			return x
		}
	`
	eval(t, src, []byte("FOO"))
}
