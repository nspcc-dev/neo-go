package vm

import (
	"bytes"
	"encoding/binary"
	"testing"
)

var src = `
package NEP5 

func Main() int {
	x := 1
	y := 2
	z := 5 + 5
	arr := []int{1, 2, 3}
	str := "anthony"
	arrStr := []string{"a", "b"}
	guess := y

	return x + y
}
`

// opener
//
// 0x52 push len arguments
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
// 0x62 jump
// 0x03 bytes pushed on the stack
// 0x00 put empty array on the stack
// 0x59 put the number 9 on the stack
// 0x6C put the input onto the main stack remove from alt
// 0x76 dup the item on top of the stack
// 0x6B put the item on the alt stack

func TestCompilerDebug(t *testing.T) {
	c := NewCompiler()
	script := []byte(src)
	if err := c.Compile(bytes.NewReader(script)); err != nil {
		t.Fatal(err)
	}
	//str := hex.EncodeToString(c.sb.buf.Bytes())

	buf := new(bytes.Buffer)

	binary.Write(buf, binary.LittleEndian, c.sb.buf.Bytes())

	t.Log(buf.Bytes())
}
