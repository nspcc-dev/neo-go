package compiler

import (
	"strings"
	"testing"
)

func TestSimpleAssign(t *testing.T) {
	src := `
		package NEP5	

		func Main() {
		}

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
	if want, have := 2, len(c.funcs); want != have {
		t.Fatalf("expected exacly %d function got %d", want, have)
	}
	_, ok := c.funcs["binaryOp"]
	if !ok {
		t.Fatal("compiler should have func (binaryOp) in its scope")
	}
	if want, have := 4, len(c.funcs["binaryOp"].scope); want != have {
		t.Fatalf("expected %d got %d", want, have)
	}
}

func TestSimpleAssign34(t *testing.T) {
	src := `
		package NEP5	

		func Main() int {
			//x := callFunc()
			//y := someOtherFunc()
			tokenName := "NEO"
			if getTokenName() == tokenName {
				return 100
			}
			return 10
		}

		// func callFunc() int {
		// 	x := 10 + someOtherFunc()
		// 	return x
		// }

		// func someOtherFunc() int {
		// 	x := 10
		// 	return x
		// }

		func getTokenName() string {
			return "NEO"
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
