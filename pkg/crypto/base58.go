package crypto

import (
	"bytes"

	"github.com/CityOfZion/neo-go/pkg/crypto/hash"
	"github.com/mr-tron/base58"
	"github.com/pkg/errors"
)

// Base58CheckDecode decodes the given string.
func Base58CheckDecode(s string) (b []byte, err error) {
	b, err = base58.Decode(s)
	if err != nil {
		return nil, err
	}

	for i := 0; i < len(s); i++ {
		if s[i] != '1' {
			break
		}
		b = append([]byte{0x00}, b...)
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

// Base58CheckEncode encodes b into a base-58 check encoded string.
func Base58CheckEncode(b []byte) string {
	b = append(b, hash.Checksum(b)...)

	return base58.Encode(b)
}
