package pubkeytesthelper

import (
	"testing"

	"github.com/CityOfZion/neo-go/pkg/crypto/hash"
	"github.com/CityOfZion/neo-go/pkg/crypto/privatekey"
	"github.com/stretchr/testify/assert"
)

func TestPubKeyVerify(t *testing.T) {
	actual, err := SignDataWithRandomPrivateKey([]byte("sample"))

	if err != nil {
		t.Fatal(err)
	}
	expected := true

	assert.Equal(t, expected, actual)
}

func TestWrongPubKey(t *testing.T) {
	privKey, _ := privatekey.NewPrivateKey()
	sample := []byte("sample")
	hashedData, _ := hash.Sha256(sample)
	signedData, _ := privKey.Sign(sample)

	secondPrivKey, _ := privatekey.NewPrivateKey()
	wrongPubKey, _ := secondPrivKey.PublicKey()

	actual := wrongPubKey.Verify(signedData, hashedData.Bytes())
	expcted := false
	assert.Equal(t, expcted, actual)
}
