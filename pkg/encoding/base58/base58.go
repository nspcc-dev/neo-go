package base58

import (
	"bytes"
	"errors"

	"github.com/mr-tron/base58"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
)

// CheckDecode implements base58-encoded string decoding with a hash-based
// checksum check.
func CheckDecode(s string) (b []byte, err error) {
	b, err = base58.Decode(s)
	if err != nil {
		return nil, err
	}

	if len(b) < 5 {
		return nil, errors.New("invalid base-58 check string: missing checksum")
	}

	if !bytes.Equal(hash.Checksum(b[:len(b)-4]), b[len(b)-4:]) {
		return nil, errors.New("invalid base-58 check string: invalid checksum")
	}

	// Strip the 4 byte long hash.
	b = b[:len(b)-4]

	return b, nil
}

// CheckEncode encodes the given byte slice into a base58 string with a hash-based
// checksum appended to it.
func CheckEncode(b []byte) string {
	b = append(b, hash.Checksum(b)...)

	return base58.Encode(b)
}
