package stack

import (
	"bytes"
	"errors"
	"math/big"
	"strconv"
)

// ByteArray represents a slice of bytes on the stack
type ByteArray struct {
	*abstractItem
	val []byte
}

//NewByteArray returns a ByteArray stack item
// given a byte slice
func NewByteArray(val []byte) *ByteArray {
	return &ByteArray{
		&abstractItem{},
		val,
	}
}

//ByteArray overrides the default abstractItem Bytes array method
func (ba *ByteArray) ByteArray() (*ByteArray, error) {
	return ba, nil
}

//Equals returns true, if two bytearrays are equal
func (ba *ByteArray) Equals(other *ByteArray) *Boolean {
	// If either are nil, return false
	if ba == nil || other == nil {
		return NewBoolean(false)
	}
	return NewBoolean(bytes.Equal(ba.val, other.val))
}

//Integer overrides the default Integer method to convert an
// ByteArray Into an integer
func (ba *ByteArray) Integer() (*Int, error) {
	dest := reverse(ba.val)
	integerVal := new(big.Int).SetBytes(dest)
	return NewInt(integerVal)

}

// Boolean will convert a byte array into a boolean stack item
func (ba *ByteArray) Boolean() (*Boolean, error) {
	boolean, err := strconv.ParseBool(string(ba.val))
	if err != nil {
		return nil, errors.New("cannot convert byte array to a boolean")
	}
	return NewBoolean(boolean), nil
}

// XXX: move this into a pkg/util/slice folder
// Go mod not working
func reverse(b []byte) []byte {
	if len(b) < 2 {
		return b
	}

	dest := make([]byte, len(b))

	for i, j := 0, len(b)-1; i < j+1; i, j = i+1, j-1 {
		dest[i], dest[j] = b[j], b[i]
	}

	return dest
}
