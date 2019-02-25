package pubkeytesthelper

import (
	"github.com/CityOfZion/neo-go/pkg/crypto/hash"
	"github.com/CityOfZion/neo-go/pkg/crypto/privatekey"
)

func SignDataWithRandomPrivateKey(data []byte) (bool, error) {

	hashedData, _ := hash.Sha256(data)

	privKey, _ := privatekey.NewPrivateKey()
	signedData, err := privKey.Sign(data)
	pubKey, _ := privKey.PublicKey()
	result := pubKey.Verify(signedData, hashedData.Bytes())
	if err != nil {
		return false, err
	}
	return result, nil
}
