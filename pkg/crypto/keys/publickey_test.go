package keys

import (
	"encoding/hex"
	"testing"

	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/stretchr/testify/assert"
)

func TestEncodeDecodeInfinity(t *testing.T) {
	key := &PublicKey{}
	buf := io.NewBufBinWriter()
	assert.Nil(t, key.EncodeBinary(buf.BinWriter))
	assert.Nil(t, buf.Err)
	b := buf.Bytes()
	assert.Equal(t, 1, len(b))

	keyDecode := &PublicKey{}
	assert.Nil(t, keyDecode.DecodeBytes(b))
	assert.Equal(t, []byte{0x00}, keyDecode.Bytes())
}

func TestEncodeDecodePublicKey(t *testing.T) {
	for i := 0; i < 4; i++ {
		k, err := NewPrivateKey()
		assert.Nil(t, err)
		p := k.PublicKey()
		buf := io.NewBufBinWriter()
		assert.Nil(t, p.EncodeBinary(buf.BinWriter))
		assert.Nil(t, buf.Err)
		b := buf.Bytes()

		pDecode := &PublicKey{}
		assert.Nil(t, pDecode.DecodeBytes(b))
		assert.Equal(t, p.X, pDecode.X)
	}
}

func TestDecodeFromString(t *testing.T) {
	str := "03b209fd4f53a7170ea4444e0cb0a6bb6a53c2bd016926989cf85f9b0fba17a70c"
	pubKey, err := NewPublicKeyFromString(str)
	assert.Nil(t, err)
	assert.Equal(t, str, hex.EncodeToString(pubKey.Bytes()))
}

func TestPubkeyToAddress(t *testing.T) {
	pubKey, err := NewPublicKeyFromString("031ee4e73a17d8f76dc02532e2620bcb12425b33c0c9f9694cc2caa8226b68cad4")
	assert.Nil(t, err)
	actual := pubKey.Address()
	expected := "AUpGsNCHzSimeMRVPQfhwrVdiUp8Q2N2Qx"
	assert.Equal(t, expected, actual)
}
