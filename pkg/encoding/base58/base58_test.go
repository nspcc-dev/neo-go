package base58

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckEncodeDecode(t *testing.T) {
	var b58CsumEncoded = "KxhEDBQyyEFymvfJD96q8stMbJMbZUb6D1PmXqBWZDU2WvbvVs9o"
	var b58CsumDecodedHex = "802bfe58ab6d9fd575bdc3a624e4825dd2b375d64ac033fbc46ea79dbab4f69a3e01"

	b58CsumDecoded, _ := hex.DecodeString(b58CsumDecodedHex)
	encoded := CheckEncode(b58CsumDecoded)
	decoded, err := CheckDecode(b58CsumEncoded)
	assert.Nil(t, err)
	assert.Equal(t, encoded, b58CsumEncoded)
	assert.Equal(t, decoded, b58CsumDecoded)
}

func TestCheckDecodeFailures(t *testing.T) {
	badbase58 := "BASE%*"
	_, err := CheckDecode(badbase58)
	assert.NotNil(t, err)
	shortbase58 := "THqY"
	_, err = CheckDecode(shortbase58)
	assert.NotNil(t, err)
	badcsum := "KxhEDBQyyEFymvfJD96q8stMbJMbZUb6D1PmXqBWZDU2WvbvVs9A"
	_, err = CheckDecode(badcsum)
	assert.NotNil(t, err)
}
