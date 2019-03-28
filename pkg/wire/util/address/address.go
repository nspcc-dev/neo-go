package address

import (
	"encoding/hex"

	"github.com/CityOfZion/neo-go/pkg/crypto/base58"
	"github.com/CityOfZion/neo-go/pkg/wire/util"
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

// FromUint160 returns the "NEO address" from the given
// Uint160.
func FromUint160(u util.Uint160) (string, error) {
	// Dont forget to prepend the Address version 0x17 (23) A
	b := append([]byte{0x17}, u.Bytes()...)
	return base58.CheckEncode(b)
}

// Uint160Decode attempts to decode the given NEO address string
// into an Uint160.
func Uint160Decode(s string) (u util.Uint160, err error) {
	b, err := base58.CheckDecode(s)
	if err != nil {
		return u, err
	}
	return util.Uint160DecodeBytes(b[1:21])
}
