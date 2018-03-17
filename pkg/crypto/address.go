package crypto

import (
	"github.com/CityOfZion/neo-go/pkg/util"
)

// AddressFromUint160 returns the "NEO address" from the given
// Uint160.
func AddressFromUint160(u util.Uint160) string {
	// Dont forget to prepend the Address version 0x17 (23) A
	b := append([]byte{0x17}, u.Bytes()...)
	return Base58CheckEncode(b)
}

// Uint160DecodeAddress attempts to decode the given NEO address string
// into an Uint160.
func Uint160DecodeAddress(s string) (u util.Uint160, err error) {
	b, err := Base58CheckDecode(s)
	if err != nil {
		return u, err
	}
	return util.Uint160DecodeBytes(b[1:21])
}
