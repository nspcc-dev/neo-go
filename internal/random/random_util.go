package random

import (
	"math/rand"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// String returns a random string with the n as its length.
func String(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(Int(65, 90))
	}

	return string(b)
}

// Bytes returns a random byte slice of specified length.
func Bytes(n int) []byte {
	b := make([]byte, n)
	Fill(b)
	return b
}

// Fill fills buffer with random bytes.
func Fill(buf []byte) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	// Rand reader returns no errors
	r.Read(buf)
}

// Int returns a random integer in [min,max).
func Int(min, max int) int {
	return min + rand.Intn(max-min)
}

// Uint256 returns a random Uint256.
func Uint256() util.Uint256 {
	str := String(20)
	return hash.Sha256([]byte(str))
}

// Uint160 returns a random Uint160.
func Uint160() util.Uint160 {
	str := String(20)
	return hash.RipeMD160([]byte(str))
}

func init() {
	//nolint:staticcheck
	rand.Seed(time.Now().UTC().UnixNano())
}
