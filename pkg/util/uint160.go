package util

import (
	"encoding/hex"
)

const uint160Size = 20

// Uint160 is a 20 byte long unsigned integer.
type Uint160 [uint160Size]uint8

// ToSlice returns a byte slice of u.
func (u Uint160) ToSlice() []byte {
	b := make([]byte, uint160Size)
	for i := 0; i < uint160Size; i++ {
		b[i] = byte(u[i])
	}
	return b
}

// String implements the stringer interface.
func (u Uint160) String() string {
	return hex.EncodeToString(u.ToSlice())
}

// Equals returns true if both Uint256 values are the same.
func (u Uint160) Equals(other Uint160) bool {
	return u.String() == other.String()
}
