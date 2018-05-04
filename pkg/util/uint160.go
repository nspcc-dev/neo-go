package util

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"golang.org/x/crypto/ripemd160"
)

const uint160Size = 20

// Uint160 is a 20 byte long unsigned integer.
type Uint160 [uint160Size]uint8

// Uint160DecodeString attempts to decode the given string into an Uint160.
func Uint160DecodeString(s string) (u Uint160, err error) {
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
	for i := 0; i < uint160Size; i++ {
		u[i] = b[i]
	}
	return
}

// Uint160FromScript returns a Uint160 type from a raw script.
func Uint160FromScript(script []byte) (u Uint160, err error) {
	sha := sha256.New()
	sha.Write(script)
	b := sha.Sum(nil)
	ripemd := ripemd160.New()
	ripemd.Write(b)
	b = ripemd.Sum(nil)
	return Uint160DecodeBytes(b)
}

// Bytes returns the byte slice representation of u.
func (u Uint160) Bytes() []byte {
	b := make([]byte, uint160Size)
	for i := 0; i < uint160Size; i++ {
		b[i] = byte(u[i])
	}
	return b
}

// BytesReverse return a reversed byte representation of u.
func (u Uint160) BytesReverse() []byte {
	return ArrayReverse(u.Bytes())
}

// String implements the stringer interface.
func (u Uint160) String() string {
	return hex.EncodeToString(u.Bytes())
}

// Equals returns true if both Uint256 values are the same.
func (u Uint160) Equals(other Uint160) bool {
	for i := 0; i < uint160Size; i++ {
		if u[i] != other[i] {
			return false
		}
	}
	return true
}

// UnmarshalJSON implements the json unmarshaller interface.
func (u *Uint160) UnmarshalJSON(data []byte) (err error) {
	var js string
	if err = json.Unmarshal(data, &js); err != nil {
		return err
	}
	if strings.HasPrefix(js, "0x") {
		js = js[2:]
	}
	*u, err = Uint160DecodeString(js)
	return err
}

// MarshalJSON implements the json marshaller interface.
func (u Uint160) MarshalJSON() ([]byte, error) {
	return json.Marshal(
		fmt.Sprintf("0x%s", hex.EncodeToString(ArrayReverse(u.Bytes()))),
	)
}
