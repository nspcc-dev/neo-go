package math

import "github.com/nspcc-dev/neo-go/pkg/interop/neogointernal"

// Pow returns a^b using POW VM opcode.
// b must be >= 0 and <= 2^31-1.
func Pow(a, b int) int {
	return neogointernal.Opcode2("POW", a, b).(int)
}

// Sqrt returns positive square root of x rounded down.
func Sqrt(x int) int {
	return neogointernal.Opcode1("SQRT", x).(int)
}
