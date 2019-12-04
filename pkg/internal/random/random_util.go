package random

import (
	"math/rand"
	"time"

	"github.com/CityOfZion/neo-go/pkg/crypto/hash"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// String returns a random string with the n as its length.
func String(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(Int(65, 90))
	}

	return string(b)
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
	rand.Seed(time.Now().UTC().UnixNano())
}
