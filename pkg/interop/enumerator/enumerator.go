/*
Package enumerator provides functions to work with enumerators.
*/
package enumerator

// Enumerator represents NEO enumerator type, it's an opaque data structure
// that can be used with functions from this package. It's similar to more
// widely used Iterator (see `iterator` package), but ranging over arrays
// or structures that have values with no explicit keys.
type Enumerator struct{}

// Create creates a new enumerator from the given items (slice, structure, byte
// array and integer or boolean converted to byte array). New enumerator points
// at index -1 of its items, so the user of it has to advance it first with Next.
// This function uses `System.Enumerator.Create` syscall.
func Create(items interface{}) Enumerator {
	return Enumerator{}
}

// Next moves position of the given enumerator by one and returns a bool that
// tells whether there is a new value present in this new position. If it is,
// you can use Value to get it, if not then there are no more values in this
// enumerator. This function uses `System.Enumerator.Next` syscall.
func Next(e Enumerator) bool {
	return true
}

// Value returns current enumerator's item value, it's only valid to call it
// after Next returning true. This function uses `System.Enumerator.Value` syscall.
func Value(e Enumerator) interface{} {
	return nil
}
