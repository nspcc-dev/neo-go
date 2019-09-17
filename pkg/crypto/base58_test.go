package crypto

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBase58CheckEncodeDecode(t *testing.T) {
	var b58CsumEncoded = "KxhEDBQyyEFymvfJD96q8stMbJMbZUb6D1PmXqBWZDU2WvbvVs9o"
	var b58CsumDecodedHex = "802bfe58ab6d9fd575bdc3a624e4825dd2b375d64ac033fbc46ea79dbab4f69a3e01"

	b58CsumDecoded, _ := hex.DecodeString(b58CsumDecodedHex)
	encoded := Base58CheckEncode(b58CsumDecoded)
	decoded, err := Base58CheckDecode(b58CsumEncoded)
	assert.Nil(t, err)
	assert.Equal(t, encoded, b58CsumEncoded)
	assert.Equal(t, decoded, b58CsumDecoded)
}

func TestBase58CheckDecodeFailures(t *testing.T) {
	badbase58 := "BASE%*"
	_, err := Base58CheckDecode(badbase58)
	assert.NotNil(t, err)
	shortbase58 := "THqY"
	_, err = Base58CheckDecode(shortbase58)
	assert.NotNil(t, err)
	badcsum := "KxhEDBQyyEFymvfJD96q8stMbJMbZUb6D1PmXqBWZDU2WvbvVs9A"
	_, err = Base58CheckDecode(badcsum)
	assert.NotNil(t, err)
}
