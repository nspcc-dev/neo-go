package vm_test

import (
	"math/big"
	"testing"
)

func TestSimpleFunctionCall(t *testing.T) {
	src := `
		package testcase
		func Main() int {
			x := 10
			y := getSomeInteger()
			return x + y
		}

		func getSomeInteger() int {
			x := 10
			return x
		}
	`
	eval(t, src, big.NewInt(20))
}

func TestNotAssignedFunctionCall(t *testing.T) {
	src := `
		package testcase
		func Main() int {
			getSomeInteger()
			getSomeInteger()
			return 0
		}

		func getSomeInteger() int {
			return 0
		}
	`
	eval(t, src, big.NewInt(0))
}

func TestMultipleFunctionCalls(t *testing.T) {
	src := `
		package testcase
		func Main() int {
			x := 10
			y := getSomeInteger()
			return x + y
		}

		func getSomeInteger() int {
			x := 10
			y := getSomeOtherInt()
			return x + y
		}

		func getSomeOtherInt() int {
			x := 8
			return x
		}
	`
	eval(t, src, big.NewInt(28))
}

func TestFunctionCallWithArgs(t *testing.T) {
	src := `
		package testcase
		func Main() int {
			x := 10
			y := getSomeInteger(x)
			return y
		}

		func getSomeInteger(x int) int {
			y := 8
			return x + y
		}
	`
	eval(t, src, big.NewInt(18))
}

func TestFunctionCallWithInterfaceType(t *testing.T) {
	src := `
		package testcase
		func Main() interface{} {
			x := getSomeInteger(10)
			return x
		}

		func getSomeInteger(x interface{}) interface{} {
			return x
		}
	`
	eval(t, src, big.NewInt(10))
}

func TestFunctionCallMultiArg(t *testing.T) {
	src := `
		package testcase
		func Main() int {
			x := addIntegers(2, 4)
			return x
		}

		func addIntegers(x int, y int) int {
			return x + y
		}
	`
	eval(t, src, big.NewInt(6))
}

func TestFunctionWithVoidReturn(t *testing.T) {
	src := `
		package testcase
		func Main() int {
			x := 2
			getSomeInteger()
			y := 4
			return x + y
		}

		func getSomeInteger() { }
	`
	eval(t, src, big.NewInt(6))
}
