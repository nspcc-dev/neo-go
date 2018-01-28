package util

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
)

// Uint256 ...
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

// ToSlice return a byte slice of u.
func (u Uint256) ToSlice() []byte {
	b := make([]byte, 32)
	for i := 0; i < 32; i++ {
		b[i] = byte(u[i])
	}
	return b
}

func (u Uint256) String() string {
	return hex.EncodeToString(u.ToSlice())
}
