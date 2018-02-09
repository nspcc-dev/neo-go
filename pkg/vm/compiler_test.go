package vm

import (
	"strings"
	"testing"
)

// opener
//
// 0x53 push len arguments
// 0xc5 open new array
// 0x6B to alt stack
//
// 0x5A the number 10 is pushed on the stack
// 0x6C put the input onto the main stack remove from alt
// 0x76 dup the item on top of the stack
// 0x6B put the item on the alt stack
// 0x00 put empty array on the stack
// 0x52 put the number 2 on the stack
// 0x7A put the item n back on top of the stack
// 0xC4 set item
// 0x59 put the number 9 on the stack
// 0x6C put the input onto the main stack remove from alt stack
// 0x76 dup the item on top of the stackj
// 0x6B put the item on the alt stack
// 0x51 push the number 1 on the stack
// 0x52 push the number 2 on the stack
// 0x7A put the item n back on top of the stack
// 0xC4 set the item
// 0x62 JMP
// 0x03 the next 3 bytes is dat pushed on the stack
// 0x6C put the input ont the main stack remove from alt stack
// 0x00 put empty array onto the stack
// 0x02 the next 2 bytes is data pushed on the stack
// 0xE8 1000 uint16
// 0x03 1000 uint16
// 0x6C put the input onto the main stack remove from alt
// 0x76 dup the item on top of the stack
// 0x6B put the item on the alt stack
// 0x52 push the number 2 on the stack
// 0x52 push the number 2 on the stack
// 0x7A put the item n back on top of the stack
// 0xC4 set the item
// 0x00 empty array is pushed on the stack
// 0x61 nop
// 0x6C put the input onto the main stack remove from alt
// 0x75 removes the top stack item
// 0x66 return

//

func TestSimpleAssign(t *testing.T) {
	src := `
		package NEP5	

		func Main() {
			x := 10
			y := 8
		}
	`

	c := NewCompiler()
	if err := c.Compile(strings.NewReader(src)); err != nil {
		t.Fatal(err)
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
