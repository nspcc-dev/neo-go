package stack

import (
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

//Integer overrides the default Integer method to convert an
// ByteArray Into an integer
func (ba *ByteArray) Integer() (*Int, error) {

	dest := make([]byte, 0)

	for i, j := 0, len(ba.val)-1; i < j+1; i, j = i+1, j-1 {
		dest[i], dest[j] = ba.val[j], ba.val[i]
	}

	integerVal := new(big.Int).SetBytes(dest)

	return &Int{
		ba.abstractItem,
		integerVal,
	}, nil

	// return ba, nil
}

// Boolean will convert
func (ba *ByteArray) Boolean() (*Boolean, error) {
	boolean, err := strconv.ParseBool(string(ba.val))
	if err != nil {
		return nil, errors.New("cannot convert byte array to a boolean")
	}
	return &Boolean{
		ba.abstractItem,
		boolean,
	}, nil
}
