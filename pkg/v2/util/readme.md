## Package - Utils


Still unsure about how to organise the uint160 and uint256 files. Trying to avoid uint256.Uint256


in the uint256 the following function reverses bytes, however I am unsure about why:

`
func Uint256DecodeBytes(b []byte) (u Uint256, err error) {
	// b = slice.Reverse(b)
	if len(b) != uint256Size {
		return u, fmt.Errorf("expected []byte of size %d got %d", uint256Size, len(b))
	}
	for i := 0; i < uint256Size; i++ {
		u[i] = b[i]
	}
	return u, nil
}
`