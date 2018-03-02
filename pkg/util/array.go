package util

// ArrayReverse return a reversed version of the given byte slice.
func ArrayReverse(b []byte) []byte {
	// Protect from big.Ints that have 1 len bytes.
	if len(b) < 2 {
		return b
	}
	dest := make([]byte, len(b))
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		dest[i], dest[j] = b[j], b[i]
	}
	return dest
}
