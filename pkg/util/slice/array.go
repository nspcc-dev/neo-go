/*
Package slice contains byte slice helpers.
*/
package slice

// CopyReverse returns a new byte slice containing reversed version of the
// original.
func CopyReverse(b []byte) []byte {
	dest := make([]byte, len(b))
	for i, j := 0, len(b)-1; i <= j; i, j = i+1, j-1 {
		dest[i], dest[j] = b[j], b[i]
	}
	return dest
}
