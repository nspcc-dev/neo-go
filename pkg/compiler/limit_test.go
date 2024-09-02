package compiler_test

import (
	"bytes"
	"fmt"
	"math/big"
	"testing"
)

// Test for #605, #623.
// Codegen should emit integers in proper format.
func TestManyVariables(t *testing.T) {
	// any number with MSB=1 is suitable
	// 155 was in the contract where this bug was first found.
	const count = 155

	buf := bytes.NewBufferString("package main\n")
	for i := range count {
		buf.WriteString(fmt.Sprintf("var a%d = %d\n", i, i))
	}
	buf.WriteString("func Main() int {\nreturn 7\n}\n")

	src := buf.String()

	eval(t, src, big.NewInt(7))
}
