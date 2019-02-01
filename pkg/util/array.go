package util

// ArrayReverse return a reversed version of the given byte slice.
func ArrayReverse(arr []byte) []byte {
	// Protect from big.Ints that have 1 len bytes.
	if len(arr) < 2 {
		return arr
	}
	for i := len(arr)/2 - 1; i >= 0; i-- {
		opp := len(arr) - 1 - i
		arr[i], arr[opp] = arr[opp], arr[i]
	}
	return arr
}
