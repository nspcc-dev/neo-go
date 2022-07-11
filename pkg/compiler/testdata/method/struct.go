package method

// X is some type.
type X struct {
	a int
}

// GetA returns the value of a.
func (x X) GetA() int {
	return x.a
}

// NewX creates a new X instance.
func NewX() X {
	return X{a: 42}
}
