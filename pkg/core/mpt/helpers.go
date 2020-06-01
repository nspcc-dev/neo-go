package mpt

import "github.com/nspcc-dev/neo-go/pkg/util"

// lcp returns longest common prefix of a and b.
// Note: it does no allocations.
func lcp(a, b []byte) []byte {
	if len(a) < len(b) {
		return lcp(b, a)
	}

	var i int
	for i = 0; i < len(b); i++ {
		if a[i] != b[i] {
			break
		}
	}

	return a[:i]
}

// copySlice is a helper for copying slice if needed.
func copySlice(a []byte) []byte {
	b := make([]byte, len(a))
	copy(b, a)
	return b
}

// toNibbles mangles path by splitting every byte into 2 containing low- and high- 4-byte part.
func toNibbles(path []byte) []byte {
	result := make([]byte, len(path)*2)
	for i := range path {
		result[i*2] = path[i] >> 4
		result[i*2+1] = path[i] & 0x0F
	}
	return result
}

// ToNeoStorageKey converts storage key to C# neo node's format.
// Key is expected to be at least 20 bytes in length.
// our format: script hash in BE + key
// neo format: script hash in LE + key with 0 between every 16 bytes, padded to len 16.
func ToNeoStorageKey(key []byte) []byte {
	const groupSize = 16

	var nkey []byte
	for i := util.Uint160Size - 1; i >= 0; i-- {
		nkey = append(nkey, key[i])
	}

	key = key[util.Uint160Size:]

	index := 0
	remain := len(key)
	for remain >= groupSize {
		nkey = append(nkey, key[index:index+groupSize]...)
		nkey = append(nkey, 0)
		index += groupSize
		remain -= groupSize
	}

	if remain > 0 {
		nkey = append(nkey, key[index:]...)
	}

	padding := groupSize - remain
	for i := 0; i < padding; i++ {
		nkey = append(nkey, 0)
	}
	return append(nkey, byte(padding))
}
