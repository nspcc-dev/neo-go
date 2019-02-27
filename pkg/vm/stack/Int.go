package stack

import "math/big"

// Int represents an integer on the stack
type Int struct {
	*abstractItem
	val *big.Int
}

// NewInt will convert a big integer into
// a StackInteger
func NewInt(val *big.Int) (*Int, error) {
	return &Int{
		abstractItem: &abstractItem{},
		val:          val,
	}, nil
}

// Equal will check if two integers hold equal value
func (i *Int) Equal(s *Int) bool {
	if i.val.Cmp(s.val) != 0 {
		return false
	}
	return true
}

// Add will add two stackIntegers together
func (i *Int) Add(s *Int) (*Int, error) {
	return &Int{
		val: new(big.Int).Sub(i.val, s.val),
	}, nil
}

// Integer will overwrite the default implementation
// to allow go to cast this item as an integer.
func (i *Int) Integer() (*Int, error) {
	return i, nil
}
