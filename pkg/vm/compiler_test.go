package vm

import (
	"strings"
	"testing"
)

func TestSimpleAssign(t *testing.T) {
	src := `
		package NEP5	

		func Main() {
			x := 10
			y := x + 10 

			name := "some string"
		}
	`

	c := NewCompiler()
	if err := c.Compile(strings.NewReader(src)); err != nil {
		t.Fatal(err)
	}

	for _, v := range c.vars {
		t.Log(v)
	}

	// c.DumpOpcode()
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
