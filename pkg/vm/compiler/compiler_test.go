package compiler

import (
	"fmt"
	"strings"
	"testing"
)

func TestResolveBinaryExpr(t *testing.T) {
	src := `
		package NEP5	
		func binExpr(z int, otherZ int) int {
			x := 8 + 4 / 2
			return x
		}
	`
	c := New()
	if err := c.Compile(strings.NewReader(src)); err != nil {
		t.Fatal(err)
	}
	fun := c.funcs["binExpr"]
	ctx := fun.getContext("x")
	if have, want := ctx.value.(int64), int64(10); want != have {
		t.Fatalf("expected %d got %d", want, have)
	}
}

func TestResolveBinaryExprWithIdent(t *testing.T) {
	src := `
		package NEP5	
		func binExpr(z int, otherZ int) int {
			x := 8 + 4 / 2 // 10
			y := 10 + x // 20
			return y
		}
	`
	c := New()
	if err := c.Compile(strings.NewReader(src)); err != nil {
		t.Fatal(err)
	}
	fun := c.funcs["binExpr"]
	ctx := fun.getContext("y")
	if have, want := ctx.value.(int64), int64(20); want != have {
		t.Fatalf("expected %d got %d", want, have)
	}
}

func TestSimpleAssign(t *testing.T) {
	src := `
		package NEP5	

		func someInt(z int, otherZ int) int {
			x := 3
			if 3 < 5 {
				x = 5
			}
			return x
		}

	`

	c := New()
	if err := c.Compile(strings.NewReader(src)); err != nil {
		t.Fatal(err)
	}

	for _, funcCTX := range c.funcs {
		for _, v := range funcCTX.vars {
			fmt.Println(v)
		}
	}

	//c.DumpOpcode()
}
