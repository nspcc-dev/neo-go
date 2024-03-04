/*
Package slice contains byte slice helpers.
*/
package slice

// CopyReverse returns a new byte slice containing reversed version of the
// original.
func CopyReverse(b []byte) []byte {
	dest := make([]byte, len(b))
	reverse(dest, b)
	return dest
}

// Reverse does in-place reversing of byte slice.
func Reverse(b []byte) {
	reverse(b, b)
}

func reverse(dst []byte, src []byte) {
	for i, j := 0, len(src)-1; i <= j; i, j = i+1, j-1 {
		dst[i], dst[j] = src[j], src[i]
	}
}

// Clean wipes the data in b by filling it with zeros.
func Clean(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
