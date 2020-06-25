/*
Package crypto provides an interface to VM cryptographic instructions and syscalls.
*/
package crypto

// SHA1 computes SHA1 hash of b. It uses `SHA1` VM instruction.
func SHA1(b []byte) []byte {
	return nil
}

// SHA256 computes SHA256 hash of b. It uses `SHA256` VM instruction.
func SHA256(b []byte) []byte {
	return nil
}

// Hash160 computes SHA256 + RIPEMD-160 of b, which is commonly used for
// script hashing and address generation. It uses `HASH160` VM instruction.
func Hash160(b []byte) []byte {
	return nil
}

// Hash256 computes double SHA256 hash of b (SHA256(SHA256(b))) which is used
// as ID for transactions, blocks and assets. It uses `HASH256` VM instruction.
func Hash256(b []byte) []byte {
	return nil
}

// VerifySignature checks that sig is correct msg's signature for a given pub
// (serialized public key). It uses `VERIFY` VM instruction.
func VerifySignature(msg []byte, sig []byte, pub []byte) bool {
	return false
}

// Secp256k1Recover recovers public key from the given signature (r, s) on the
// given message hash using Secp256k1 elliptic curve. Flag isEven denotes Y's
// least significant bit in decompression algorithm. The return value is byte
// array representation of the public key which is either empty (if it's not
// possible to recover key) or contains 64 bytes with X and Y coordinates (both
// 32-byte in BE format). This function uses Neo.Cryptography.Secp256k1Recover
// syscall.
func Secp256k1Recover(r []byte, s []byte, messageHash []byte, isEven bool) []byte {
	return nil
}

// Secp256r1Recover recovers public key from the given signature (r, s) on the
// given message hash using Secp256r1 elliptic curve. Flag isEven denotes Y's
// least significant bit in decompression algorithm.  The return value is byte
// array representation of the public key which is either empty (if it's not
// possible to recover key) or contains 64 bytes with X and Y coordinates (both
// 32-byte in BE format). This function uses Neo.Cryptography.Secp256r1Recover
// syscall.
func Secp256r1Recover(r []byte, s []byte, messageHash []byte, isEven bool) []byte {
	return nil
}
