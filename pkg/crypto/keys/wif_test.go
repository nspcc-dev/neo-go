package keys

import (
	"encoding/hex"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/encoding/base58"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type wifTestCase struct {
	wif        string
	compressed bool
	privateKey string
	version    byte
}

var wifTestCases = []wifTestCase{
	{
		wif:        "KwDiBf89QgGbjEhKnhXJuH7LrciVrZi3qYjgd9M7rFU73sVHnoWn",
		compressed: true,
		privateKey: "0000000000000000000000000000000000000000000000000000000000000001",
		version:    0x80,
	},
	{
		wif:        "5HpHagT65TZzG1PH3CSu63k8DbpvD8s5ip4nEB3kEsreAnchuDf",
		compressed: false,
		privateKey: "0000000000000000000000000000000000000000000000000000000000000001",
		version:    0x80,
	},
	{
		wif:        "KxhEDBQyyEFymvfJD96q8stMbJMbZUb6D1PmXqBWZDU2WvbvVs9o",
		compressed: true,
		privateKey: "2bfe58ab6d9fd575bdc3a624e4825dd2b375d64ac033fbc46ea79dbab4f69a3e",
		version:    0x80,
	},
	{
		wif:        "KxhEDBQyyEFymvfJD96q8stMbJMbZUb6D1PmXqBWZDU2WvbvVs9o",
		compressed: true,
		privateKey: "2bfe58ab6d9fd575bdc3a624e4825dd2b375d64ac033fbc46ea79dbab4f69a3e",
		version:    0x00,
	},
}

func TestWIFEncodeDecode(t *testing.T) {
	for _, testCase := range wifTestCases {
		b, err := hex.DecodeString(testCase.privateKey)
		assert.Nil(t, err)
		wif, err := WIFEncode(b, testCase.version, testCase.compressed)
		assert.Nil(t, err)
		assert.Equal(t, testCase.wif, wif)

		WIF, err := WIFDecode(wif, testCase.version)
		assert.Nil(t, err)
		assert.Equal(t, testCase.privateKey, WIF.PrivateKey.String())
		assert.Equal(t, testCase.compressed, WIF.Compressed)
		if testCase.version != 0 {
			assert.Equal(t, testCase.version, WIF.Version)
		} else {
			assert.EqualValues(t, WIFVersion, WIF.Version)
		}
	}

	wifInv := []byte{0, 1, 2}
	_, err := WIFEncode(wifInv, 0, true)
	require.Error(t, err)
}

func TestBadWIFDecode(t *testing.T) {
	_, err := WIFDecode("garbage", 0)
	require.Error(t, err)

	s := base58.CheckEncode([]byte{})
	_, err = WIFDecode(s, 0)
	require.Error(t, err)

	uncompr := make([]byte, 33)
	compr := make([]byte, 34)

	s = base58.CheckEncode(compr)
	_, err = WIFDecode(s, 0)
	require.Error(t, err)

	s = base58.CheckEncode(uncompr)
	_, err = WIFDecode(s, 0)
	require.Error(t, err)

	compr[33] = 1
	compr[0] = WIFVersion
	uncompr[0] = WIFVersion

	s = base58.CheckEncode(compr)
	_, err = WIFDecode(s, 0)
	require.NoError(t, err)

	s = base58.CheckEncode(uncompr)
	_, err = WIFDecode(s, 0)
	require.NoError(t, err)
}
