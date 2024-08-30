package fee

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
)

const feeFactor = 30

// The most common Opcode() use case is to get price for a single opcode.
func BenchmarkOpcode1(t *testing.B) {
	// Just so that we don't always test the same opcode.
	script := []opcode.Opcode{opcode.NOP, opcode.ADD, opcode.SYSCALL, opcode.APPEND}
	l := len(script)
	for n := range t.N {
		_ = Opcode(feeFactor, script[n%l])
	}
}
