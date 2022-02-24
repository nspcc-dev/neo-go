package util

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util/slice"
)

// Uint160Size is the size of Uint160 in bytes.
const Uint160Size = 20

// Uint160 is a 20 byte long unsigned integer.
type Uint160 [Uint160Size]uint8

// Uint160DecodeStringBE attempts to decode the given string into an Uint160.
func Uint160DecodeStringBE(s string) (Uint160, error) {
	var u Uint160
	if len(s) != Uint160Size*2 {
		return u, fmt.Errorf("expected string size of %d got %d", Uint160Size*2, len(s))
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return u, err
	}
	return Uint160DecodeBytesBE(b)
}

// Uint160DecodeStringLE attempts to decode the given string
// in little-endian hex encoding into an Uint160.
func Uint160DecodeStringLE(s string) (Uint160, error) {
	var u Uint160
	if len(s) != Uint160Size*2 {
		return u, fmt.Errorf("expected string size of %d got %d", Uint160Size*2, len(s))
	}

	b, err := hex.DecodeString(s)
	if err != nil {
		return u, err
	}

	return Uint160DecodeBytesLE(b)
}

// Uint160DecodeBytesBE attempts to decode the given bytes into an Uint160.
func Uint160DecodeBytesBE(b []byte) (u Uint160, err error) {
	if len(b) != Uint160Size {
		return u, fmt.Errorf("expected byte size of %d got %d", Uint160Size, len(b))
	}
	copy(u[:], b)
	return
}

// Uint160DecodeBytesLE attempts to decode the given bytes in little-endian
// into an Uint160.
func Uint160DecodeBytesLE(b []byte) (u Uint160, err error) {
	if len(b) != Uint160Size {
		return u, fmt.Errorf("expected byte size of %d got %d", Uint160Size, len(b))
	}

	for i := range b {
		u[Uint160Size-i-1] = b[i]
	}

	return
}

// BytesBE returns a big-endian byte representation of u.
func (u Uint160) BytesBE() []byte {
	return u[:]
}

// BytesLE returns a little-endian byte representation of u.
func (u Uint160) BytesLE() []byte {
	return slice.CopyReverse(u.BytesBE())
}

// String implements the stringer interface.
func (u Uint160) String() string {
	return u.StringBE()
}

// StringBE returns string representations of u with big-endian byte order.
func (u Uint160) StringBE() string {
	return hex.EncodeToString(u.BytesBE())
}

// StringLE returns string representations of u with little-endian byte order.
func (u Uint160) StringLE() string {
	return hex.EncodeToString(u.BytesLE())
}

// Reverse returns reversed representation of u.
func (u Uint160) Reverse() (r Uint160) {
	for i := 0; i < Uint160Size; i++ {
		r[i] = u[Uint160Size-i-1]
	}

	return
}

// Equals returns true if both Uint160 values are the same.
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
	*u, err = Uint160DecodeStringLE(js)
	return err
}

// MarshalJSON implements the json marshaller interface.
func (u Uint160) MarshalJSON() ([]byte, error) {
	return []byte(`"0x` + u.StringLE() + `"`), nil
}

// UnmarshalYAML implements the YAML Unmarshaler interface.
func (u *Uint160) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string

	err := unmarshal(&s)
	if err != nil {
		return err
	}

	s = strings.TrimPrefix(s, "0x")
	*u, err = Uint160DecodeStringLE(s)
	return err
}

// MarshalYAML implements the YAML marshaller interface.
func (u Uint160) MarshalYAML() (interface{}, error) {
	return "0x" + u.StringLE(), nil
}

// EncodeBinary implements Serializable interface.
func (u *Uint160) EncodeBinary(bw *io.BinWriter) {
	bw.WriteBytes(u[:])
}

// DecodeBinary implements Serializable interface.
func (u *Uint160) DecodeBinary(br *io.BinReader) {
	br.ReadBytes(u[:])
}
