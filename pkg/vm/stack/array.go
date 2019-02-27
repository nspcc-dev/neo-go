package stack

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
