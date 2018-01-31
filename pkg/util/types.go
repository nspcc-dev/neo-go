package util

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
)

// Uint256 is a 32 byte long unsigned integer.
// Commonly used to store hashes.
type Uint256 [32]uint8

// Uint256FromBytes return an Uint256 from a byte slice.
func Uint256FromBytes(b []byte) Uint256 {
	if len(b) != 32 {
		err := fmt.Sprintf("%d does not match the size of Uint256 (32 bytes)", len(b))
		panic(err)
	}

	var val [32]uint8
	for i := 0; i < 32; i++ {
		val[i] = b[i]
	}

	return Uint256(val)
}

// UnmarshalBinary implements the Binary Unmarshaler interface.
func (u *Uint256) UnmarshalBinary(b []byte) error {
	r := bytes.NewReader(b)
	binary.Read(r, binary.LittleEndian, u)
	return nil
}

// ToSlice returns a byte slice of u.
func (u Uint256) ToSlice() []byte {
	b := make([]byte, 32)
	for i := 0; i < len(b); i++ {
		b[i] = byte(u[i])
	}
	return b
}

func (u Uint256) String() string {
	return hex.EncodeToString(u.ToSlice())
}

// Uint160 is a 20 byte long unsigned integer
type Uint160 [20]uint8

// ToSlice returns a byte slice of u.
func (u Uint160) ToSlice() []byte {
	b := make([]byte, 20)
	for i := 0; i < len(b); i++ {
		b[i] = byte(u[i])
	}
	return b
}

// String implements the stringer interface.
func (u Uint160) String() string {
	return hex.EncodeToString(u.ToSlice())
}
