package vm

import (
	"strings"
	"testing"
)

func TestSimpleAssign(t *testing.T) {
	src := `
		package NEP5	

		func Main() {
			x := 4 + 2
			name := "anthony"
			y := x + 2
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
