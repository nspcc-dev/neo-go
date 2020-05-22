package base58

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestBase58LeadingZeroes(t *testing.T) {
	buf := []byte{0, 0, 0, 1}
	b58 := CheckEncode(buf)
	dec, err := CheckDecode(b58)
	require.NoError(t, err)
	require.Equal(t, buf, dec)
}
