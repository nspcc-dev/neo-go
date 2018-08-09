package address

import (
	"encoding/hex"

	"github.com/CityOfZion/neo-go/pkg/wire/util/crypto/base58"
)

func ToScriptHash(address string) string {

	decodedAddressAsBytes, err := base58.Decode(address)
	if err != nil {
		return ""
	}
	decodedAddressAsHex := hex.EncodeToString(decodedAddressAsBytes)
	scriptHash := (decodedAddressAsHex[2:42])
	return scriptHash
}
