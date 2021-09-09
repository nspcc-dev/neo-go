/*
Package iterator provides functions to work with Neo iterators.
*/
package iterator

import "github.com/nspcc-dev/neo-go/pkg/interop/neogointernal"

// Iterator represents a Neo iterator, it's an opaque data structure that can
// be properly created by Create or storage.Find. Iterators range over key-value
// pairs, so it's convenient to use them for maps. This structure is similar in
// function to Neo .net framework's Iterator.
type Iterator struct{}

// Next advances the iterator returning true if it was successful (and you
// can use Value to get value for slices or key-value pair for maps) and false
// otherwise (and there are no more elements in this Iterator). This function
// uses `System.Iterator.Next` syscall.
func Next(it Iterator) bool {
	return neogointernal.Syscall1("System.Iterator.Next", it).(bool)
}

// Value returns iterator's current value. It's only valid to call after
// successful Next call. This function uses `System.Iterator.Value` syscall.
// For slices the result is just value.
// For maps the result can be casted to a slice of 2 elements: key and value.
// For storage iterators refer to `storage.FindFlags` documentation.
func Value(it Iterator) interface{} {
	return neogointernal.Syscall1("System.Iterator.Value", it)
}
