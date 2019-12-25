package address

import (
	"github.com/CityOfZion/neo-go/pkg/encoding/base58"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// EncodeUint160 returns the "NEO address" from the given Uint160.
func EncodeUint160(u util.Uint160) string {
	// Dont forget to prepend the Address version 0x17 (23) A
	b := append([]byte{0x17}, u.BytesBE()...)
	return base58.CheckEncode(b)
}

// DecodeUint160 attempts to decode the given NEO address string
// into an Uint160.
func DecodeUint160(s string) (u util.Uint160, err error) {
	b, err := base58.CheckDecode(s)
	if err != nil {
		return u, err
	}
	return util.Uint160DecodeBytesBE(b[1:21])
}
