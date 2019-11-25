package testutil

import (
	"math/rand"
	"time"

	"github.com/CityOfZion/neo-go/pkg/crypto/hash"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// RandomString returns a random string with the n as its length.
func RandomString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(RandomInt(65, 90))
	}

	return string(b)
}

// RandomInt returns a random integer in [min,max).
func RandomInt(min, max int) int {
	return min + rand.Intn(max-min)
}

// RandomUint256 returns a random Uint256.
func RandomUint256() util.Uint256 {
	str := RandomString(20)
	return hash.Sha256([]byte(str))
}

// RandomUint160 returns a random Uint160.
func RandomUint160() util.Uint160 {
	str := RandomString(20)
	return hash.RipeMD160([]byte(str))
}

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}
