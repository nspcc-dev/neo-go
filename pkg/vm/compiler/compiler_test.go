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
	if fun, ok := c.funcs["binaryOp"]; !ok {
		if fun.label != 0 {
			t.Fatalf("expected label of the function to be %d got %d", 0, fun.label)
		}
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
			x := 10
			if x < 10 {
				return 5
			}
			return x
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
