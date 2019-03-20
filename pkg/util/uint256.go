package util

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
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
	b = ArrayReverse(b)
	if len(b) != uint256Size {
		return u, fmt.Errorf("expected []byte of size %d got %d", uint256Size, len(b))
	}
	copy(u[:], b)
	return u, nil
}

// Bytes returns a byte slice representation of u.
func (u Uint256) Bytes() []byte {
	return u[:]
}

// BytesReverse return a reversed byte representation of u.
func (u Uint256) BytesReverse() []byte {
	return ArrayReverse(u.Bytes())
}

// Equals returns true if both Uint256 values are the same.
func (u Uint256) Equals(other Uint256) bool {
	return u == other
}

// String implements the stringer interface.
func (u Uint256) String() string {
	return hex.EncodeToString(ArrayReverse(u.Bytes()))
}

// UnmarshalJSON implements the json unmarshaller interface.
func (u *Uint256) UnmarshalJSON(data []byte) (err error) {
	var js string
	if err = json.Unmarshal(data, &js); err != nil {
		return err
	}
	js = strings.TrimPrefix(js, "0x")
	*u, err = Uint256DecodeString(js)
	return err
}

// Size returns the lenght of the bytes representation of Uint256
func (u Uint256) Size() int {
	return uint256Size
}

// MarshalJSON implements the json marshaller interface.
func (u Uint256) MarshalJSON() ([]byte, error) {
	return []byte(`"0x` + u.String() + `"`), nil
}

// CompareTo compares two Uint256 with each other. Possible output: 1, -1, 0
//  1 implies u > other.
// -1 implies u < other.
//  0 implies  u = other.
func (u Uint256) CompareTo(other Uint256) int { return bytes.Compare(u[:], other[:]) }
