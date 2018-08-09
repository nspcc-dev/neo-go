package util

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/CityOfZion/neo-go/pkg/wire/util/slice"
)

const uint256Size = 32

// Uint256 is a 32 byte long unsigned integer.
type Uint256 [uint256Size]uint8

// Uint256DecodeString attempts to decode the given string into an Uint256.
func Uint256DecodeString(s string) (u Uint256, err error) {
	if len(s) != uint256Size*2 {
		return u, fmt.Errorf("expected string size of %d got %d", uint256Size*2, len(s))
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return u, err
	}
	return Uint256DecodeBytes(b)
}

// Uint256DecodeBytes attempts to decode the given string into an Uint256.
func Uint256DecodeBytes(b []byte) (u Uint256, err error) {
	if len(b) != uint256Size {
		return u, fmt.Errorf("expected []byte of size %d got %d", uint256Size, len(b))
	}
	for i := 0; i < uint256Size; i++ {
		u[i] = b[i]
	}
	return u, nil
}

// Bytes returns a byte slice representation of u.
func (u Uint256) Bytes() []byte {
	b := make([]byte, uint256Size)
	for i := 0; i < uint256Size; i++ {
		b[i] = byte(u[i])
	}
	return b
}
func (u Uint256) Reverse() (res Uint256) {
	res, _ = Uint256DecodeBytes(u.BytesReverse())
	return
}

// BytesReverse return a reversed byte representation of u.
func (u Uint256) BytesReverse() []byte {
	return slice.Reverse(u.Bytes())
}

// Equals returns true if both Uint256 values are the same.
func (u Uint256) Equals(other Uint256) bool {
	return u.String() == other.String()
}

// String implements the stringer interface.
func (u Uint256) String() string {
	return hex.EncodeToString(slice.Reverse(u.Bytes()))
}

// MarshalJSON implements the json marshaller interface.
func (u Uint256) MarshalJSON() ([]byte, error) {
	return json.Marshal(fmt.Sprintf("0x%s", u.String()))
}
