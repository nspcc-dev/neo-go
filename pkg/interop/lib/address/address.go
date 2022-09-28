package address

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/std"
	"github.com/nspcc-dev/neo-go/pkg/interop/runtime"
)

// ToHash160 is a utility function that converts a Neo address to its hash
// (160 bit BE value in a 20 byte slice). When parameter is known at compile time
// (it's a constant string) the output is calculated by the compiler and this
// function is optimized out completely. Otherwise, standard library and system
// calls are used to perform the conversion and checks (panic will happen on
// invalid input).
func ToHash160(address string) interop.Hash160 {
	b := std.Base58CheckDecode([]byte(address))
	if len(b) != interop.Hash160Len+1 {
		panic("invalid address length")
	}
	if int(b[0]) != runtime.GetAddressVersion() {
		panic("invalid address prefix")
	}
	return b[1:21]
}

// FromHash160 is a utility function that converts given Hash160 to
// Base58-encoded Neo address.
func FromHash160(hash interop.Hash160) string {
	if len(hash) != interop.Hash160Len {
		panic("invalid Hash160 length")
	}
	res := append([]byte{byte(runtime.GetAddressVersion())}, hash...)
	return std.Base58CheckEncode(res)
}
