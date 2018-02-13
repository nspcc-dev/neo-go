package compiler

import (
	"fmt"
	"strings"
	"testing"
)

func TestSimpleAssign(t *testing.T) {
	src := `
		package NEP5	

		func binaryOp() int {
			x := 5
			y := x
			z := y
			u := z
			return u
		}
	`

	c := New()
	if err := c.Compile(strings.NewReader(src)); err != nil {
		t.Fatal(err)
	}
	if want, have := 1, len(c.funcs); want != have {
		t.Fatalf("expected %d got %d", want, have)
	}
	if _, ok := c.funcs["binaryOp"]; !ok {
		t.Fatal("compiler should have func (binaryOp) in its scope")
	}
	if want, have := 4, len(c.funcs["binaryOp"].scope); want != have {
		t.Fatalf("expected %d got %d", want, have)
	}
}

func TestSimpleAssign34(t *testing.T) {
	src := `
		package NEP5	

		func binaryOp() int {
			x := 5
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
