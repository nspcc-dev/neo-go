package keys

import (
	"testing"

	"github.com/CityOfZion/neo-go/pkg/crypto/hash"
	"github.com/stretchr/testify/assert"
)

func TestPubKeyVerify(t *testing.T) {
	var data = []byte("sample")
	hashedData := hash.Sha256(data)

	privKey, err := NewPrivateKey()
	assert.Nil(t, err)
	signedData, err := privKey.Sign(data)
	assert.Nil(t, err)
	pubKey := privKey.PublicKey()
	result := pubKey.Verify(signedData, hashedData.BytesBE())
	expected := true
	assert.Equal(t, expected, result)

	pubKey = &PublicKey{}
	assert.False(t, pubKey.Verify(signedData, hashedData.BytesBE()))
}

func TestWrongPubKey(t *testing.T) {
	privKey, _ := NewPrivateKey()
	sample := []byte("sample")
	hashedData := hash.Sha256(sample)
	signedData, _ := privKey.Sign(sample)

	secondPrivKey, _ := NewPrivateKey()
	wrongPubKey := secondPrivKey.PublicKey()

	actual := wrongPubKey.Verify(signedData, hashedData.BytesBE())
	expcted := false
	assert.Equal(t, expcted, actual)
}
