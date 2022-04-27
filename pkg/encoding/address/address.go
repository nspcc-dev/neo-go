package address

import (
	"errors"

	"github.com/nspcc-dev/neo-go/pkg/encoding/base58"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

const (
	// NEO2Prefix is the first byte of an address for NEO2.
	NEO2Prefix byte = 0x17
	// NEO3Prefix is the first byte of an address for NEO3.
	NEO3Prefix byte = 0x35
)

// Prefix is the byte used to prepend to addresses when encoding them, it can
// be changed and defaults to 53 (0x35), the standard NEO prefix.
var Prefix = NEO3Prefix

// Uint160ToString returns the "NEO address" from the given Uint160.
func Uint160ToString(u util.Uint160) string {
	// Don't forget to prepend the Address version 0x17 (23) A
	b := append([]byte{Prefix}, u.BytesBE()...)
	return base58.CheckEncode(b)
}

// StringToUint160 attempts to decode the given NEO address string
// into a Uint160.
func StringToUint160(s string) (u util.Uint160, err error) {
	b, err := base58.CheckDecode(s)
	if err != nil {
		return u, err
	}
	if b[0] != Prefix {
		return u, errors.New("wrong address prefix")
	}
	return util.Uint160DecodeBytesBE(b[1:21])
}
