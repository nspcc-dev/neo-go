/*
Package crypto provides an interface to cryptographic syscalls.
*/
package crypto

// SHA256 computes SHA256 hash of b. It uses `Neo.Crypto.SHA256` syscall.
func SHA256(b []byte) []byte {
	return nil
}

// ECDsaVerify checks that sig is correct msg's signature for a given pub
// (serialized public key). It uses `Neo.Crypto.ECDsaVerify` syscall.
func ECDsaVerify(msg []byte, pub []byte, sig []byte) bool {
	return false
}
