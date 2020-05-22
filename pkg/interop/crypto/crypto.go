package crypto

// Package crypto provides function signatures that can be used inside
// smart contracts that are written in the neo-go framework.

// SHA256 computes the sha256 hash of b.
func SHA256(b []byte) []byte {
	return nil
}

// ECDsaVerify checks that sig is msg's signature with pub.
func ECDsaVerify(msg []byte, pub []byte, sig []byte) bool {
	return false
}
