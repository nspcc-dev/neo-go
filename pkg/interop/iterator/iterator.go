/*
Package iterator provides functions to work with Neo iterators.
*/
package iterator

import "github.com/nspcc-dev/neo-go/pkg/interop/enumerator"

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

// Key returns iterator's key at current position. It's only valid to call after
// successful Next call. This function uses `System.Iterator.Key` syscall.
func Key(it Iterator) interface{} {
	return nil
}

// Keys returns Enumerator ranging over keys or the given Iterator. Note that
// this Enumerator is actually directly tied to the underlying Iterator, so that
// advancing it with Next will actually advance the Iterator too. This function
// uses `System.Iterator.Keys` syscall.
func Keys(it Iterator) enumerator.Enumerator {
	return enumerator.Enumerator{}
}

// Next advances the iterator returning true if it is was successful (and you
// can use Key or Value) and false otherwise (and there are no more elements in
// this Iterator). This function uses `System.Enumerator.Next` syscall.
func Next(it Iterator) bool {
	return true
}

// Value returns iterator's current value. It's only valid to call after
// successful Next call. This function uses `System.Enumerator.Value` syscall.
func Value(it Iterator) interface{} {
	return nil
}

// Values returns Enumerator ranging over values or the given Iterator. Note that
// this Enumerator is actually directly tied to the underlying Iterator, so that
// advancing it with Next will actually advance the Iterator too. This function
// uses `System.Iterator.Values` syscall.
func Values(it Iterator) enumerator.Enumerator {
	return enumerator.Enumerator{}
}
