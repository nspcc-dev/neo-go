package stack

import (
	"fmt"
)

// Array represents an Array of stackItems on the stack
type Array struct {
	*abstractItem
	val []Item
}

// Array overrides the default implementation
// by the abstractItem, returning an Array struct
func (a *Array) Array() (*Array, error) {
	return a, nil
}

//Value returns the underlying Array's value
func (a *Array) Value() []Item {
	return a.val
}

// NewArray returns a new Array.
func NewArray(val []Item) (*Array, error) {
	return &Array{
		&abstractItem{},
		val,
	}, nil
}

// Hash overrides the default abstract hash method.
func (a *Array) Hash() (string, error) {
	data := fmt.Sprintf("%T %v", a, a.Value())
	return KeyGenerator([]byte(data))
}
