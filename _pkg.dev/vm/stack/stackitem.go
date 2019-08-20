package stack

import (
	"errors"
)

//Item is an interface which represents object that can be placed on the stack
type Item interface {
	Integer() (*Int, error)
	Boolean() (*Boolean, error)
	ByteArray() (*ByteArray, error)
	Array() (*Array, error)
	Context() (*Context, error)
	Map() (*Map, error)
	Hash() (string, error)
}

// Represents an `abstract` stack item
// which will hold default values for stack items
// this is intended to be embedded into types that you will use on the stack
type abstractItem struct{}

// Integer is the default implementation for a stackItem
// Implements Item interface
func (a *abstractItem) Integer() (*Int, error) {
	return nil, errors.New("This stack item is not an Integer")
}

// Boolean is the default implementation for a stackItem
// Implements Item interface
func (a *abstractItem) Boolean() (*Boolean, error) {
	return nil, errors.New("This stack item is not a Boolean")
}

// ByteArray is the default implementation for a stackItem
// Implements Item interface
func (a *abstractItem) ByteArray() (*ByteArray, error) {
	return nil, errors.New("This stack item is not a byte array")
}

// Array is the default implementation for a stackItem
// Implements Item interface
func (a *abstractItem) Array() (*Array, error) {
	return nil, errors.New("This stack item is not an array")
}

// Context is the default implementation for a stackItem
// Implements Item interface
func (a *abstractItem) Context() (*Context, error) {
	return nil, errors.New("This stack item is not of type context")
}

// Context is the default implementation for a stackItem
// Implements Item interface
func (a *abstractItem) Map() (*Map, error) {
	return nil, errors.New("This stack item is not a map")
}

func (a *abstractItem) Hash() (string, error) {
	return "", errors.New("This stack item need to override the Hash Method")
}
