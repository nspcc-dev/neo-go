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

// Uint256DecodeReverseString attempts to decode the given string (in LE representation) into an Uint256.
func Uint256DecodeReverseString(s string) (u Uint256, err error) {
	if len(s) != uint256Size*2 {
		return u, fmt.Errorf("expected string size of %d got %d", uint256Size*2, len(s))
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return u, err
	}
	return Uint256DecodeReverseBytes(b)
}

// Uint256DecodeBytes attempts to decode the given string (in BE representation) into an Uint256.
func Uint256DecodeBytes(b []byte) (u Uint256, err error) {
	if len(b) != uint256Size {
		return u, fmt.Errorf("expected []byte of size %d got %d", uint256Size, len(b))
	}
	copy(u[:], b)
	return u, nil
}

// Uint256DecodeReverseBytes attempts to decode the given string (in LE representation) into an Uint256.
func Uint256DecodeReverseBytes(b []byte) (u Uint256, err error) {
	b = ArrayReverse(b)
	return Uint256DecodeBytes(b)
}

// Bytes returns a byte slice representation of u.
func (u Uint256) Bytes() []byte {
	return u[:]
}

// Reverse reverses the Uint256 object
func (u Uint256) Reverse() Uint256 {
	res, _ := Uint256DecodeReverseBytes(u.Bytes())
	return res
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
	return hex.EncodeToString(u.Bytes())
}

// ReverseString produces string representation of Uint256 with LE byte order.
func (u Uint256) ReverseString() string {
	return hex.EncodeToString(u.BytesReverse())
}

// UnmarshalJSON implements the json unmarshaller interface.
func (u *Uint256) UnmarshalJSON(data []byte) (err error) {
	var js string
	if err = json.Unmarshal(data, &js); err != nil {
		return err
	}
	js = strings.TrimPrefix(js, "0x")
	*u, err = Uint256DecodeReverseString(js)
	return err
}

// MarshalJSON implements the json marshaller interface.
func (u Uint256) MarshalJSON() ([]byte, error) {
	return []byte(`"0x` + u.ReverseString() + `"`), nil
}

// CompareTo compares two Uint256 with each other. Possible output: 1, -1, 0
//  1 implies u > other.
// -1 implies u < other.
//  0 implies  u = other.
func (u Uint256) CompareTo(other Uint256) int { return bytes.Compare(u[:], other[:]) }
