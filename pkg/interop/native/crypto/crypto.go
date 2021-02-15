package crypto

import (
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
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
	return contract.Call(interop.Hash160(Hash), "sha256", contract.NoneFlag, b).(interop.Hash256)
}

// Ripemd160 calls `ripemd160` method of native CryptoLib contract and computes RIPEMD160 hash of b.
func Ripemd160(b []byte) interop.Hash160 {
	return contract.Call(interop.Hash160(Hash), "ripemd160", contract.NoneFlag, b).(interop.Hash160)
}

// VerifyWithECDsa calls `verifyWithECDsa` method of native CryptoLib contract and checks that sig is
// correct msg's signature for a given pub (serialized public key on a given curve).
func VerifyWithECDsa(msg []byte, pub interop.PublicKey, sig interop.Signature, curve NamedCurve) bool {
	return contract.Call(interop.Hash160(Hash), "verifyWithECDsa", contract.NoneFlag, msg, pub, sig, curve).(bool)
}
