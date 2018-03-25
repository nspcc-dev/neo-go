package crypto

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncodeDecodeInfinity(t *testing.T) {
	key := &PublicKey{ECPoint{}}
	buf := new(bytes.Buffer)
	assert.Nil(t, key.EncodeBinary(buf))
	assert.Equal(t, 1, buf.Len())

	keyDecode := &PublicKey{}
	assert.Nil(t, keyDecode.DecodeBinary(buf))
	assert.Equal(t, []byte{0x00}, keyDecode.Bytes())
}

func TestEncodeDecodePublicKey(t *testing.T) {
	for i := 0; i < 4; i++ {
		p := &PublicKey{RandomECPoint()}
		buf := new(bytes.Buffer)
		assert.Nil(t, p.EncodeBinary(buf))

		pDecode := &PublicKey{}
		assert.Nil(t, pDecode.DecodeBinary(buf))
		assert.Equal(t, p.X, pDecode.X)
	}
}

func TestDecodeFromString(t *testing.T) {
	str := "03b209fd4f53a7170ea4444e0cb0a6bb6a53c2bd016926989cf85f9b0fba17a70c"
	pubKey, err := NewPublicKeyFromString(str)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, str, hex.EncodeToString(pubKey.Bytes()))
}
