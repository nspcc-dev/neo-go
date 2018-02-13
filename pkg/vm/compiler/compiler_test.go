package compiler

import (
	"strings"
	"testing"
)

var testCases = []string{
	`
	package testcase
	func Main() int {
		x := 2 + 2
		return x
	}
	`,
	`
	package testcase
	func Main() int {
		x := 2
		y := 4
		return x + y
	}
	`,
	`
	package testcase
	func Main() int {
		x := 10 - 2
		y := 4 * x
		return y
	}
	`,
	`
	package testcase
	func Main() int {
		x := 10 - 2 / 2
		y := 4 * x - 10
		return y
	}
	`,
	`
	package testcase
	func Main() string {
		tokenName := "foo"
		return tokenName
	}
	`,
	`
	package testcase
	func Main() string {
		tokenName := "foo"
		x := 2
		if x < 2 {
			return tokenName
		}
		return "something else" 
	}
	`,
}

func TestAllCases(t *testing.T) {
	for _, src := range testCases {
		if err := New().Compile(strings.NewReader(src)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSimpleAssign34(t *testing.T) {
	src := `
		package NEP5	

		func Main() int {
			x := true
			if x {
				return  5
			}
			return 10
		}
	`

	c := New()
	if err := c.Compile(strings.NewReader(src)); err != nil {
		t.Fatal(err)
	}

	// for _, c := range c.funcCalls {
	// 	fmt.Println(c)
	// }

	// for _, fctx := range c.funcs {
	// 	fmt.Println(fctx.label)
	// 	for _, v := range fctx.scope {
	// 		fmt.Println(v)
	// 	}
	// }

	c.DumpOpcode()
}
