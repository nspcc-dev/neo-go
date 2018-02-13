package compiler

import (
	"fmt"
	"strings"
	"testing"
)

func TestSimpleAssign(t *testing.T) {
	src := `
		package NEP5	

		func someInt(z int, otherZ int) int {
			x := 5
			// y := 5 / 5 + x + 10 - x
			y := x / 5 + 6 - 1
			return y 
		}

	`

	c := New()
	if err := c.Compile(strings.NewReader(src)); err != nil {
		t.Fatal(err)
	}

	for _, fctx := range c.funcs {
		for _, v := range fctx.scope {
			fmt.Println(v)
		}
	}

	c.DumpOpcode()
}
