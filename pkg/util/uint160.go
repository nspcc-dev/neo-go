package util

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

const uint160Size = 20

// Uint160 is a 20 byte long unsigned integer.
type Uint160 [uint160Size]uint8

// Uint160DecodeString attempts to decode the given string into an Uint160.
func Uint160DecodeString(s string) (Uint160, error) {
	var u Uint160
	if len(s) != uint160Size*2 {
		return u, fmt.Errorf("expected string size of %d got %d", uint160Size*2, len(s))
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return u, err
	}
	return Uint160DecodeBytes(b)
}

// Uint160DecodeBytes attempts to decode the given bytes into an Uint160.
func Uint160DecodeBytes(b []byte) (u Uint160, err error) {
	if len(b) != uint160Size {
		return u, fmt.Errorf("expected byte size of %d got %d", uint160Size, len(b))
	}
	copy(u[:], b)
	return
}

// Bytes returns the byte slice representation of u.
func (u Uint160) Bytes() []byte {
	return u[:]
}

// BytesReverse returns a reversed byte representation of u.
func (u Uint160) BytesReverse() []byte {
	return ArrayReverse(u.Bytes())
}

// String implements the stringer interface.
func (u Uint160) String() string {
	return hex.EncodeToString(u.Bytes())
}

// ReverseString is the same as String, but returns a reversed representation.
func (u Uint160) ReverseString() string {
	return hex.EncodeToString(u.BytesReverse())
}

// Equals returns true if both Uint256 values are the same.
func (u Uint160) Equals(other Uint160) bool {
	return u == other
}

// Less returns true if this value is less than given Uint160 value. It's
// primarily intended to be used for sorting purposes.
func (u Uint160) Less(other Uint160) bool {
	for k := range u {
		if u[k] == other[k] {
			continue
		}
		return u[k] < other[k]
	}
	return false
}

// UnmarshalJSON implements the json unmarshaller interface.
func (u *Uint160) UnmarshalJSON(data []byte) (err error) {
	var js string
	if err = json.Unmarshal(data, &js); err != nil {
		return err
	}
	js = strings.TrimPrefix(js, "0x")
	*u, err = Uint160DecodeString(js)
	return err
}

// MarshalJSON implements the json marshaller interface.
func (u Uint160) MarshalJSON() ([]byte, error) {
	return []byte(`"0x` + u.String() + `"`), nil
}
