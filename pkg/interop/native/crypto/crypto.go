/*
Package crypto provides interface to CryptoLib native contract.
It implements some cryptographic functions.
*/
package crypto

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/interop/neogointernal"
)

// Hash represents CryptoLib contract hash.
const Hash = "\x1b\xf5\x75\xab\x11\x89\x68\x84\x13\x61\x0a\x35\xa1\x28\x86\xcd\xe0\xb6\x6c\x72"

// NamedCurve represents named elliptic curve.
type NamedCurve byte

// Various named elliptic curves.
const (
	Secp256k1 NamedCurve = 22
	Secp256r1 NamedCurve = 23
)

// Sha256 calls `sha256` method of native CryptoLib contract and computes SHA256 hash of b.
func Sha256(b []byte) interop.Hash256 {
	return neogointernal.CallWithToken(Hash, "sha256", int(contract.NoneFlag), b).(interop.Hash256)
}

// Ripemd160 calls `ripemd160` method of native CryptoLib contract and computes RIPEMD160 hash of b.
func Ripemd160(b []byte) interop.Hash160 {
	return neogointernal.CallWithToken(Hash, "ripemd160", int(contract.NoneFlag), b).(interop.Hash160)
}

// Murmur32 calls `murmur32` method of native CryptoLib contract and computes Murmur32 hash of b
// using the given seed.
func Murmur32(b []byte, seed int) []byte {
	return neogointernal.CallWithToken(Hash, "murmur32", int(contract.NoneFlag), b, seed).([]byte)
}

// VerifyWithECDsa calls `verifyWithECDsa` method of native CryptoLib contract and checks that sig is
// correct msg's signature for a given pub (serialized public key on a given curve).
func VerifyWithECDsa(msg []byte, pub interop.PublicKey, sig interop.Signature, curve NamedCurve) bool {
	return neogointernal.CallWithToken(Hash, "verifyWithECDsa", int(contract.NoneFlag), msg, pub, sig, curve).(bool)
}
