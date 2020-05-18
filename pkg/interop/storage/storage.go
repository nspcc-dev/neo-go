/*
Package storage provides functions to access and modify contract's storage.
Neo storage's model follows simple key-value DB pattern, this storage is a part
of blockchain state, so you can use it between various invocations of the same
contract.
*/
package storage

import "github.com/nspcc-dev/neo-go/pkg/interop/iterator"

// Context represents storage context that is mandatory for Put/Get/Delete
// operations. It's an opaque type that can only be created properly by
// GetContext. It's similar to Neo .net framework's StorageContext class.
type Context struct{}

// GetContext returns current contract's (that invokes this function) storage
// context. It uses `Neo.Storage.GetContext` syscall.
func GetContext() Context { return Context{} }

// Put saves given value with given key in the storage using given Context.
// Even though it accepts interface{} for both, you can only pass simple types
// there like string, []byte, int or bool (not structures or slices of more
// complex types). To put more complex types there serialize them first using
// runtime.Serialize. This function uses `Neo.Storage.Put` syscall.
func Put(ctx Context, key interface{}, value interface{}) {}

// Get retrieves value stored for the given key using given Context. See Put
// documentation on possible key and value types. This function uses
// `Neo.Storage.Get` syscall.
func Get(ctx Context, key interface{}) interface{} { return 0 }

// Delete removes key-value pair from storage by the given key using given
// Context. See Put documentation on possible key types. This function uses
// `Neo.Storage.Delete` syscall.
func Delete(ctx Context, key interface{}) {}

// Find returns an iterator.Iterator over key-value pairs in the given Context
// that match the given key (contain it as a prefix). See Put documentation on
// possible key types and iterator package documentation on how to use the
// returned value. This function uses `Neo.Storage.Find` syscall.
func Find(ctx Context, key interface{}) iterator.Iterator { return iterator.Iterator{} }
