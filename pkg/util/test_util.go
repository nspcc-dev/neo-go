package util

import (
	"crypto/sha256"
	"math/rand"
	"time"

	"golang.org/x/crypto/ripemd160"
)

// RandomString returns a random string with the n as its length.
func RandomString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(RandomInt(65, 90))
	}

	return string(b)
}

// RandomInt returns a ramdom integer betweeen min and max.
func RandomInt(min, max int) int {
	return min + rand.Intn(max-min)
}

// RandomUint256 returns a random Uint256.
func RandomUint256() Uint256 {
	str := RandomString(20)
	h := sha256.Sum256([]byte(str))
	return Uint256(h)
}

// RandomUint160 returns a random Uint160.
func RandomUint160() Uint160 {
	str := RandomString(20)
	ripemd := ripemd160.New()
	ripemd.Write([]byte(str))
	h := ripemd.Sum(nil)
	v, _ := Uint160DecodeBytes(h)
	return v
}

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}
