package enumerator

// Package enumerator provides function signatures that can be used inside
// smart contracts that are written in the neo-storm framework.

// TODO: Check enumerator use cases and add them to the examples folder.

// Enumerator stubs a NEO enumerator type.
type Enumerator struct{}

// Create creates a new enumerator from the given items.
func Create(items []interface{}) Enumerator {
	return Enumerator{}
}

// Next returns the next item in the iteration.
func Next(e Enumerator) interface{} {
	return nil
}

// Value returns the enumerator value.
func Value(e Enumerator) interface{} {
	return nil
}

// Concat concats the 2 given enumerators.
func Concat(a, b Enumerator) Enumerator {
	return Enumerator{}
}
