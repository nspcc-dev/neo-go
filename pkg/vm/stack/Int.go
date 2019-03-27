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
		val: new(big.Int).Add(i.val, s.val),
	}, nil
}

// Sub will subtract two stackIntegers together
func (i *Int) Sub(s *Int) (*Int, error) {
	return &Int{
		val: new(big.Int).Sub(i.val, s.val),
	}, nil
}

// Mul will multiply two stackIntegers together
func (i *Int) Mul(s *Int) (*Int, error) {
	return &Int{
		val: new(big.Int).Mul(i.val, s.val),
	}, nil
}

// Mod will take the mod of two stackIntegers together
func (i *Int) Mod(s *Int) (*Int, error) {
	return &Int{
		val: new(big.Int).Mod(i.val, s.val),
	}, nil
}

// Rsh will shift the integer b to the right by `n` bits
func (i *Int) Rsh(n *Int) (*Int, error) {
	return &Int{
		val: new(big.Int).Rsh(i.val, uint(n.val.Int64())),
	}, nil
}

// Lsh will shift the integer b to the left by `n` bits
func (i *Int) Lsh(n *Int) (*Int, error) {
	return &Int{
		val: new(big.Int).Lsh(i.val, uint(n.val.Int64())),
	}, nil
}

// Integer will overwrite the default implementation
// to allow go to cast this item as an integer.
func (i *Int) Integer() (*Int, error) {
	return i, nil
}

// ByteArray override the default ByteArray method
// to convert a Integer into a byte Array
func (i *Int) ByteArray() (*ByteArray, error) {
	b := i.val.Bytes()
	dest := reverse(b)
	return NewByteArray(dest), nil
}

//Boolean override the default Boolean method
// to convert an Integer into a Boolean StackItem
func (i *Int) Boolean() (*Boolean, error) {
	boolean := (i.val.Int64() != 0)
	return NewBoolean(boolean), nil
}

//Value returns the underlying big.Int
func (i *Int) Value() *big.Int {
	return i.val
}

// Abs returns a stack integer whose underlying value is
// the absolute value of the original stack integer.
func (i *Int) Abs() (*Int, error) {
	a := i.Value()
	b, err := NewInt(a.Abs(a))
	if err != nil {
		return nil, err
	}

	return b, nil
}
