package stack

// Boolean represents a boolean value on the stack
type Boolean struct {
	*abstractItem
	val bool
}

//NewBoolean returns a new boolean stack item
func NewBoolean(val bool) *Boolean {
	return &Boolean{
		&abstractItem{},
		val,
	}
}

// Boolean overrides the default implementation
// by the abstractItem, returning a Boolean struct
func (b *Boolean) Boolean() (*Boolean, error) {
	return b, nil
}

// Value returns the underlying boolean value
func (b *Boolean) Value() bool {
	return b.val
}
