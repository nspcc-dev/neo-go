package foo

// NewBar return an integer \o/
func NewBar() int {
	return 10
}

// Dummy is dummy variable.
var Dummy = 1

// Foo is a type.
type Foo struct{}

// Bar is a function.
func Bar() int {
	return 1
}

// Bar is a method.
func (f Foo) Bar() int {
	return 8
}
