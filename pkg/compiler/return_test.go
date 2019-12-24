package compiler_test

import (
	"math/big"
	"testing"
)

func TestMultipleReturn1(t *testing.T) {
	src := `
		package hello

		func two() (int, int) {
			return 5, 9
		}

		func Main() int {
			a, _ := two()
			return a
		}
	`
	eval(t, src, big.NewInt(5))
}

func TestMultipleReturn2(t *testing.T) {
	src := `
		package hello

		func two() (int, int) {
			return 5, 9
		}

		func Main() int {
			_, b := two()
			return b
		}
	`
	eval(t, src, big.NewInt(9))
}

func TestMultipleReturnWithArg(t *testing.T) {
	src := `
		package hello
		
		func inc2(a int) (int, int) {
			return a+1, a+2 
		}

		func Main() int {
			a, b := 3, 9
			a, b = inc2(a)
			return a+b
		}
	`
	eval(t, src, big.NewInt(9))
}

func TestSingleReturn(t *testing.T) {
	src := `
		package hello

		func inc(k int) int {
			return k+1
		}

		func Main() int {
			a, b := inc(3), inc(4)
			return a+b
		}
	`
	eval(t, src, big.NewInt(9))
}
