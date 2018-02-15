package util

import (
	"encoding/hex"
	"fmt"
)

const uint256Size = 32

// Uint256 is a 32 byte long unsigned integer.
type Uint256 [uint256Size]uint8

// Uint256DecodeFromString returns an Uint256 from a (hex) string.
func Uint256DecodeFromString(s string) (Uint256, error) {
	var val Uint256

	if len(s) != uint256Size*2 {
		return val, fmt.Errorf("expected string size of %d got %d", uint256Size*2, len(s))
	}

	b, err := hex.DecodeString(s)
	if err != nil {
		return val, err
	}

	b = ToArrayReverse(b)

	return Uint256DecodeFromBytes(b)
}

// Uint256DecodeFromBytes return an Uint256 from a byte slice.
func Uint256DecodeFromBytes(b []byte) (Uint256, error) {
	var val Uint256

	if len(b) != uint256Size {
		return val, fmt.Errorf("expected []byte of size %d got %d", uint256Size, len(b))
	}

	for i := 0; i < uint256Size; i++ {
		val[i] = b[i]
	}

	return val, nil
}

// ToArrayReverse return a reversed version of the given byte slice.
func ToArrayReverse(b []byte) []byte {
	// Protect from big.Ints that have 1 len bytes.
	if len(b) < 2 {
		return b
	}

	dest := make([]byte, len(b))
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		dest[i], dest[j] = b[j], b[i]
	}

	return dest
}

// ToSlice returns a byte slice of u.
func (u Uint256) ToSlice() []byte {
	b := make([]byte, uint256Size)
	for i := 0; i < uint256Size; i++ {
		b[i] = byte(u[i])
	}
	return b
}

// ToSliceReverse returns a reversed byte slice of u.
func (u Uint256) ToSliceReverse() []byte {
	b := make([]byte, uint256Size)
	for i, j := 0, uint256Size-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = byte(u[j]), byte(u[i])
	}
	return b
}

// Equals returns true if both Uint256 values are the same.
func (u Uint256) Equals(other Uint256) bool {
	return u.String() == other.String()
}

// String implements the stringer interface.
func (u Uint256) String() string {
	return hex.EncodeToString(u.ToSliceReverse())
}
