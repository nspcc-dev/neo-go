package nested3

// Argument is used as a function argument.
var Argument = 34

// Anna is used to check struct-related usage analyzer logic (calls to methods
// and fields).
var Anna = Person{Age: 24}

// Person is an auxiliary structure containing simple field.
type Person struct {
	Age int
}

// GetAge is used to check method calls inside usage analyzer.
func (p Person) GetAge() int {
	return p.Age
}
