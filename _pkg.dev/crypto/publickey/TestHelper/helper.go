package pubkeytesthelper

import (
	"github.com/CityOfZion/neo-go/pkg/crypto/hash"
	"github.com/CityOfZion/neo-go/pkg/crypto/privatekey"
)

// SignDataWithRandomPrivateKey will sign data with
// a random private key, then verify said data
// returning true if Verify returns true
func SignDataWithRandomPrivateKey(data []byte) (bool, error) {

	hashedData, err := hash.Sha256(data)
	if err != nil {
		return false, err
	}

	privKey, err := privatekey.NewPrivateKey()
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
