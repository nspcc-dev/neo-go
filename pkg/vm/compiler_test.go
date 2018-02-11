package vm

import (
	"strings"
	"testing"
)

func TestSimpleAssign(t *testing.T) {
	src := `
		package NEP5	

		func Main() int {
			x := 19000
			y := 0

			if x < 10 {
				y = 1
			} else if x > 15 + 90 {
				y = 2
				name := "anthony"
			}


			return x
		}
	`

	c := NewCompiler()
	if err := c.Compile(strings.NewReader(src)); err != nil {
		t.Fatal(err)
	}

	for _, v := range c.vars {
		t.Log(v)
	}
	t.Log(c.vars)

	c.DumpOpcode()
}

func TestAssignLoadLocal(t *testing.T) {
	src := `
		package NEP5	

		func Main() {
			x := 1
			y := x
		}
	`

	c := NewCompiler()
	if err := c.Compile(strings.NewReader(src)); err != nil {
		t.Fatal(err)
	}

	// c.DumpOpcode()
}
