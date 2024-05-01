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

// NamedCurveHash represents a pair of named elliptic curve and hash function.
type NamedCurveHash byte

// Various pairs of named elliptic curves and hash functions.
const (
	Secp256k1Sha256    NamedCurveHash = 22
	Secp256r1Sha256    NamedCurveHash = 23
	Secp256k1Keccak256 NamedCurveHash = 24
	Secp256r1Keccak256 NamedCurveHash = 25
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
// a correct msg's signature for the given pub (serialized public key on the given curve).
func VerifyWithECDsa(msg []byte, pub interop.PublicKey, sig interop.Signature, curveHash NamedCurveHash) bool {
	return neogointernal.CallWithToken(Hash, "verifyWithECDsa", int(contract.NoneFlag), msg, pub, sig, curveHash).(bool)
}

// Bls12381Point represents BLS12-381 curve point (G1 or G2 in the Affine or
// Jacobian form or GT). Bls12381Point structure is needed for the operations
// with the curve's points (serialization, addition, multiplication, pairing and
// equality checks). It's an opaque type that can only be created properly by
// Bls12381Deserialize, Bls12381Add, Bls12381Mul or Bls12381Pairing. The only
// way to expose the Bls12381Point out of the runtime to the outside world is by
// serializing it with Bls12381Serialize method call.
type Bls12381Point struct{}

// Bls12381Serialize calls `bls12381Serialize` method of native CryptoLib contract
// and serializes given BLS12-381 point into byte array.
func Bls12381Serialize(g Bls12381Point) []byte {
	return neogointernal.CallWithToken(Hash, "bls12381Serialize", int(contract.NoneFlag), g).([]byte)
}

// Bls12381Deserialize calls `bls12381Deserialize` method of native CryptoLib
// contract and deserializes given BLS12-381 point from byte array.
func Bls12381Deserialize(data []byte) Bls12381Point {
	return neogointernal.CallWithToken(Hash, "bls12381Deserialize", int(contract.NoneFlag), data).(Bls12381Point)
}

// Bls12381Equal calls `bls12381Equal` method of native CryptoLib contract and
// checks whether two BLS12-381 points are equal.
func Bls12381Equal(x, y Bls12381Point) bool {
	return neogointernal.CallWithToken(Hash, "bls12381Equal", int(contract.NoneFlag), x, y).(bool)
}

// Bls12381Add calls `bls12381Add` method of native CryptoLib contract and
// performs addition operation over two BLS12-381 points.
func Bls12381Add(x, y Bls12381Point) Bls12381Point {
	return neogointernal.CallWithToken(Hash, "bls12381Add", int(contract.NoneFlag), x, y).(Bls12381Point)
}

// Bls12381Mul calls `bls12381Mul` method of native CryptoLib contract and
// performs multiplication operation over BLS12-381 point and the given scalar
// multiplicator. The multiplicator is the serialized LE representation of the
// field element stored on 4 words (uint64) with 32-bytes length. The last
// argument denotes whether the multiplicator should be negative.
func Bls12381Mul(x Bls12381Point, mul []byte, neg bool) Bls12381Point {
	return neogointernal.CallWithToken(Hash, "bls12381Mul", int(contract.NoneFlag), x, mul, neg).(Bls12381Point)
}

// Bls12381Pairing calls `bls12381Pairing` method of native CryptoLib contract and
// performs pairing operation over two BLS12-381 points which must be G1 and G2 either
// in Affine or Jacobian forms. The result of this operation is GT point.
func Bls12381Pairing(g1, g2 Bls12381Point) Bls12381Point {
	return neogointernal.CallWithToken(Hash, "bls12381Pairing", int(contract.NoneFlag), g1, g2).(Bls12381Point)
}

// Keccak256 calls `keccak256` method of native CryptoLib contract and
// computes Keccak256 hash of b.
func Keccak256(b []byte) interop.Hash256 {
	return neogointernal.CallWithToken(Hash, "keccak256", int(contract.NoneFlag), b).(interop.Hash256)
}
