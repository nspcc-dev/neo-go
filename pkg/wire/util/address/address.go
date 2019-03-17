package address

import (
	"encoding/hex"

	"github.com/CityOfZion/neo-go/pkg/crypto/base58"
)

// ToScriptHash converts an address to a script hash
func ToScriptHash(address string) string {

	decodedAddressAsBytes, err := base58.Decode(address)
	if err != nil {
		return ""
	}
	decodedAddressAsHex := hex.EncodeToString(decodedAddressAsBytes)
	scriptHash := (decodedAddressAsHex[2:42])
	return scriptHash
}
