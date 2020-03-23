package crypto

// Package crypto provides function signatures that can be used inside
// smart contracts that are written in the neo-go framework.

// SHA1 computes the sha1 hash of b.
func SHA1(b []byte) []byte {
	return nil
}

// SHA256 computes the sha256 hash of b.
func SHA256(b []byte) []byte {
	return nil
}

// VerifySignature checks that sig is msg's signature with pub.
func VerifySignature(msg []byte, sig []byte, pub []byte) bool {
	return false
}
