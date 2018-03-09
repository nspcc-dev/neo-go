package crypto

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"math/big"
)

const prefix rune = '1'

var decodeMap = map[rune]int64{
	'1': 0, '2': 1, '3': 2, '4': 3, '5': 4,
	'6': 5, '7': 6, '8': 7, '9': 8, 'A': 9,
	'B': 10, 'C': 11, 'D': 12, 'E': 13, 'F': 14,
	'G': 15, 'H': 16, 'J': 17, 'K': 18, 'L': 19,
	'M': 20, 'N': 21, 'P': 22, 'Q': 23, 'R': 24,
	'S': 25, 'T': 26, 'U': 27, 'V': 28, 'W': 29,
	'X': 30, 'Y': 31, 'Z': 32, 'a': 33, 'b': 34,
	'c': 35, 'd': 36, 'e': 37, 'f': 38, 'g': 39,
	'h': 40, 'i': 41, 'j': 42, 'k': 43, 'm': 44,
	'n': 45, 'o': 46, 'p': 47, 'q': 48, 'r': 49,
	's': 50, 't': 51, 'u': 52, 'v': 53, 'w': 54,
	'x': 55, 'y': 56, 'z': 57,
}

// Base58Decode decodes the base58 encoded string.
func Base58Decode(s string) ([]byte, error) {
	var (
		startIndex = 0
		zero       = 0
	)
	for i, c := range s {
		if c == prefix {
			zero++
		} else {
			startIndex = i
			break
		}
	}

	var (
		n   = big.NewInt(0)
		div = big.NewInt(58)
	)
	for _, c := range s[startIndex:] {
		charIndex, ok := decodeMap[c]
		if !ok {
			return nil, fmt.Errorf(
				"invalid character '%c' when decoding this base58 string: '%s'", c, s,
			)
		}
		n.Add(n.Mul(n, div), big.NewInt(charIndex))
	}

	out := n.Bytes()
	buf := make([]byte, (zero + len(out)))
	copy(buf[zero:], out[:])

	return buf, nil
}

// Base58Encode encodes a byte slice to be a base58 encoded string.
func Base58Encode(bytes []byte) string {
	var (
		lookupTable = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
		x           = new(big.Int).SetBytes(bytes)
		r           = new(big.Int)
		m           = big.NewInt(58)
		zero        = big.NewInt(0)
		encoded     string
	)

	for x.Cmp(zero) > 0 {
		x.QuoRem(x, m, r)
		encoded = string(lookupTable[r.Int64()]) + encoded
	}

	return encoded
}

// Base58CheckDecode decodes the given string.
func Base58CheckDecode(s string) (b []byte, err error) {
	b, err = Base58Decode(s)
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
		return nil, fmt.Errorf("Invalid base-58 check string: missing checksum.")
	}

	sha := sha256.New()
	sha.Write(b[:len(b)-4])
	hash := sha.Sum(nil)

	sha.Reset()
	sha.Write(hash)
	hash = sha.Sum(nil)

	if bytes.Compare(hash[0:4], b[len(b)-4:]) != 0 {
		return nil, fmt.Errorf("Invalid base-58 check string: invalid checksum.")
	}

	// Strip the 4 byte long hash.
	b = b[:len(b)-4]

	return b, nil
}

// Base58checkEncode encodes b into a base-58 check encoded string.
func Base58CheckEncode(b []byte) string {
	sha := sha256.New()
	sha.Write(b)
	hash := sha.Sum(nil)

	sha.Reset()
	sha.Write(hash)
	hash = sha.Sum(nil)

	b = append(b, hash[0:4]...)

	return Base58Encode(b)
}
