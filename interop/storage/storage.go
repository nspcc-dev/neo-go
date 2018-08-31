package storage

import "github.com/CityOfZion/neo-storm/interop/iterator"

// Package storage provides function signatures that can be used inside
// smart contracts that are written in the neo-storm framework.

// Context represents the storage context
type Context struct{}

// GetContext returns the storage context
func GetContext() Context { return Context{} }

// Put value at given key
func Put(ctx Context, key interface{}, value interface{}) {}

// Get value matching given key
func Get(ctx Context, key interface{}) interface{} { return 0 }

// Delete key value pair from storage
func Delete(ctx Context, key interface{}) {}

// Find returns an iterator.Iterator over the keys that matched the given key.
func Find(ctx Context, key interface{}) iterator.Iterator { return iterator.Iterator{} }
