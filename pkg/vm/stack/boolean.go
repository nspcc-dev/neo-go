package stack

// Boolean represents a boolean value on the stack
type Boolean struct {
	*abstractItem
	val bool
}

// Boolean overrides the default implementation
// by the abstractItem, returning a Boolean struct
func (b *Boolean) Boolean() (*Boolean, error) {
	return b, nil
}
