package keys

import (
	"testing"

	"github.com/CityOfZion/neo-go/pkg/crypto/hash"
	"github.com/stretchr/testify/assert"
)

// SignDataWithRandomPrivateKey will sign data with
// a random private key, then verify said data
// returning true if Verify returns true
func SignDataWithRandomPrivateKey(data []byte) (bool, error) {
	hashedData := hash.Sha256(data)

	privKey, err := NewPrivateKey()
	if err != nil {
		return false, err
	}
	signedData, err := privKey.Sign(data)
	if err != nil {
		return false, err
	}
	pubKey, err := privKey.PublicKey()
	if err != nil {
		return false, err
	}
	result := pubKey.Verify(signedData, hashedData.Bytes())

	return result, nil
}

func TestPubKeyVerify(t *testing.T) {
	actual, err := SignDataWithRandomPrivateKey([]byte("sample"))

	if err != nil {
		t.Fatal(err)
	}
	expected := true

	assert.Equal(t, expected, actual)
}

func TestWrongPubKey(t *testing.T) {
	privKey, _ := NewPrivateKey()
	sample := []byte("sample")
	hashedData := hash.Sha256(sample)
	signedData, _ := privKey.Sign(sample)

	secondPrivKey, _ := NewPrivateKey()
	wrongPubKey, _ := secondPrivKey.PublicKey()

	actual := wrongPubKey.Verify(signedData, hashedData.Bytes())
	expcted := false
	assert.Equal(t, expcted, actual)
}
