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

// Sign returns:
//
//	-1 if x <  0
//	 0 if x == 0
//	+1 if x >  0
func Sign(a int) int {
	return neogointernal.Opcode1("SIGN", a).(int)
}

// Abs returns absolute value of a.
func Abs(a int) int {
	return neogointernal.Opcode1("ABS", a).(int)
}

// Max returns maximum of a, b.
func Max(a, b int) int {
	return neogointernal.Opcode2("MAX", a, b).(int)
}

// Min returns minimum of a, b.
func Min(a, b int) int {
	return neogointernal.Opcode2("MIN", a, b).(int)
}

// Within returns true if a <= x < b.
func Within(x, a, b int) bool {
	return neogointernal.Opcode3("WITHIN", x, a, b).(bool)
}
