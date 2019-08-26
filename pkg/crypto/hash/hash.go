package hash

import (
	"crypto/sha256"

	"github.com/CityOfZion/neo-go/pkg/util"
	"golang.org/x/crypto/ripemd160"
)

// Sha256 hashes the incoming byte slice
// using the sha256 algorithm
func Sha256(data []byte) util.Uint256 {
	hash := sha256.Sum256(data)
	return hash
}

// DoubleSha256 performs sha256 twice on the given data
func DoubleSha256(data []byte) util.Uint256 {
	var hash util.Uint256

	h1 := Sha256(data)
	hash = Sha256(h1.Bytes())
	return hash
}

// RipeMD160 performs the RIPEMD160 hash algorithm
// on the given data
func RipeMD160(data []byte) util.Uint160 {
	var hash util.Uint160
	hasher := ripemd160.New()
	_, _ = hasher.Write(data)

	hash, _ = util.Uint160DecodeBytes(hasher.Sum(nil))
	return hash
}

// Hash160 performs sha256 and then ripemd160
// on the given data
func Hash160(data []byte) util.Uint160 {
	var hash util.Uint160

	h1 := Sha256(data)
	h2 := RipeMD160(h1.Bytes())
	hash, _ = util.Uint160DecodeBytes(h2.Bytes())

	return hash
}

// Checksum returns the checksum for a given piece of data
// using sha256 twice as the hash algorithm
func Checksum(data []byte) []byte {
	hash := DoubleSha256(data)
	return hash[:4]
}
