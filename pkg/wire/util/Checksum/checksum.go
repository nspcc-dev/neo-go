package checksum

import (
	"bytes"
	"encoding/binary"

	"github.com/CityOfZion/neo-go/pkg/crypto/hash"
)

// Compare calculates the checksum of b
// then compares it with the `have` checksum passed as a parameter
func Compare(have uint32, b []byte) bool {
	want := FromBytes(b)
	return have == want
}

// FromBuf calculates the checksum of a buffer
func FromBuf(buf *bytes.Buffer) uint32 {

	return FromBytes(buf.Bytes())
}

// FromBytes calculates the checksum of a byte slice
func FromBytes(buf []byte) uint32 {
	b, err := hash.DoubleSha256(buf)

	if err != nil {
		return 0
	}

	// checksum := SumSHA256(SumSHA256(buf.Bytes()))
	return binary.LittleEndian.Uint32(b.Bytes()[:4])
}
