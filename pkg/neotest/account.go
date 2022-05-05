package neotest

var _nonce uint32

// Nonce returns a unique number that can be used as a nonce for new transactions.
func Nonce() uint32 {
	_nonce++
	return _nonce
}
