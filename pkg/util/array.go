package util

// ArrayReverse returns a reversed version of the given byte slice.
func ArrayReverse(b []byte) []byte {
	dest := make([]byte, len(b))
	for i, j := 0, len(b)-1; i <= j; i, j = i+1, j-1 {
		dest[i], dest[j] = b[j], b[i]
	}
	return dest
}
