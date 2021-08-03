package opcode

import (
	"testing"
)

// IsValid() is called for every VM instruction.
func BenchmarkIsValid(t *testing.B) {
	// Just so that we don't always test the same opcode.
	script := []Opcode{NOP, ADD, SYSCALL, APPEND, 0xff, 0xf0}
	l := len(script)
	for n := 0; n < t.N; n++ {
		_ = IsValid(script[n%l])
	}
}
