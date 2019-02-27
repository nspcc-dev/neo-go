package stack

// ByteArray represents a slice of bytes on the stack
type ByteArray struct {
	*abstractItem
	val []byte
}

//ByteArray overrides the default abstractItem Bytes array method
func (ba *ByteArray) ByteArray() (*ByteArray, error) {
	return ba, nil
}
