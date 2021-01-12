/*
Package iterator provides functions to work with Neo iterators.
*/
package iterator

// Iterator represents a Neo iterator, it's an opaque data structure that can
// be properly created by Create or storage.Find. Unlike enumerators, iterators
// range over key-value pairs, so it's convenient to use them for maps. This
// structure is similar in function to Neo .net framework's Iterator.
type Iterator struct{}

// Create creates an iterator from the given items (array, struct, map, byte
// array or integer and boolean converted to byte array). A new iterator is set
// to point at element -1, so to access its first element you need to call Next
// first. This function uses `System.Iterator.Create` syscall.
func Create(items interface{}) Iterator {
	return Iterator{}
}

// Next advances the iterator returning true if it is was successful (and you
// can use Key or Value) and false otherwise (and there are no more elements in
// this Iterator). This function uses `System.Iterator.Next` syscall.
func Next(it Iterator) bool {
	return true
}

// Value returns iterator's current value. It's only valid to call after
// successful Next call. This function uses `System.Iterator.Value` syscall.
// For slices the result is just value.
// For maps the result can be casted to a slice of 2 elements: key and value.
func Value(it Iterator) interface{} {
	return nil
}
