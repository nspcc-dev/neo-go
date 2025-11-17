/*
Package math provides access to useful numeric functions available in Neo VM.
*/
package math

import "github.com/nspcc-dev/neo-go/pkg/interop/neogointernal"

// Pow returns a^b using POW VM opcode.
// b must be >= 0 and <= 2^31-1.
func Pow(a, b int) int {
	return neogointernal.Opcode2("POW", a, b).(int)
}

// Sqrt returns a positive square root of x rounded down.
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

// Abs returns an absolute value of a.
func Abs(a int) int {
	return neogointernal.Opcode1("ABS", a).(int)
}

// Within returns true if a <= x < b.
func Within(x, a, b int) bool {
	return neogointernal.Opcode3("WITHIN", x, a, b).(bool)
}

// ModMul returns the result of modulus division on a*b.
func ModMul(a, b, mod int) int {
	return neogointernal.Opcode3("MODMUL", a, b, mod).(int)
}

// ModPow returns the result of modulus division on a^b. If b is -1,
// it returns the modular inverse of a.
func ModPow(a, b, mod int) int {
	return neogointernal.Opcode3("MODPOW", a, b, mod).(int)
}
