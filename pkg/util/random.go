package util

import (
	"math/rand"
	"time"
)

// RandUint32 return a random number between min and max.
func RandUint32(min, max int) uint32 {
	n := min + rand.Intn(max-min)
	return uint32(n)
}

func init() {
	rand.Seed(time.Now().UTC().UnixNano())
}
