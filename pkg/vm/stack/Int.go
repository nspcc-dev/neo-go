package stack

import (
	"fmt"
	"math/big"
)

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

// Div will divide one stackInteger by an other.
func (i *Int) Div(s *Int) (*Int, error) {
	return &Int{
		val: new(big.Int).Div(i.val, s.val),
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
	a := big.NewInt(0).Abs(i.Value())
	b, err := NewInt(a)
	if err != nil {
		return nil, err
	}

	return b, nil
}

// Lte returns a bool value from the comparison of two integers, a and b.
// value is true if a <= b.
// value is false if a > b.
func (i *Int) Lte(s *Int) bool {
	return i.Value().Cmp(s.Value()) != 1
}

// Gte returns a bool value from the comparison of two integers, a and b.
// value is true if a >= b.
// value is false if a < b.
func (i *Int) Gte(s *Int) bool {
	return i.Value().Cmp(s.Value()) != -1
}

// Lt returns a bool value from the comparison of two integers, a and b.
// value is true if a < b.
// value is false if a >= b.
func (i *Int) Lt(s *Int) bool {
	return i.Value().Cmp(s.Value()) == -1
}

// Gt returns a bool value from the comparison of two integers, a and b.
// value is true if a > b.
// value is false if a <= b.
func (i *Int) Gt(s *Int) bool {
	return i.Value().Cmp(s.Value()) == 1
}

// Hash overrides the default abstract hash method.
func (i *Int) Hash() (string, error) {
	data := fmt.Sprintf("%T %v", i, i.Value())
	return KeyGenerator([]byte(data))

  // Min returns the mininum between two integers.
func Min(a *Int, b *Int) *Int {
	if a.Lte(b) {
		return a
	}
	return b

}

// Max returns the maximun between two integers.
func Max(a *Int, b *Int) *Int {
	if a.Gte(b) {
		return a
	}
	return b
}

// Within returns a bool whose value is true
// iff the value of the integer i is within the specified
// range [a,b) (left-inclusive).
func (i *Int) Within(a *Int, b *Int) bool {
	// i >= a && i < b
	return i.Gte(a) && i.Lt(b)
}
