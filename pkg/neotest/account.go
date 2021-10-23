package neotest

var _nonce uint32

func nonce() uint32 {
	_nonce++
	return _nonce
}
