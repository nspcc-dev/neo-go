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
// GetContext, GetReadOnlyContext or ConvertContextToReadOnly. It's similar
// to Neo .net framework's StorageContext class.
type Context struct{}

// FindFlags represents parameters to `Find` iterator.
type FindFlags byte

const (
	// None is default option. Iterator values are key-value pairs.
	None FindFlags = 0
	// KeysOnly is used for iterating over keys.
	KeysOnly FindFlags = 1 << 0
	// RemovePrefix is used for stripping 1-byte prefix from keys.
	RemovePrefix FindFlags = 1 << 1
	// ValuesOnly is used for iterating over values.
	ValuesOnly FindFlags = 1 << 2
)

// ConvertContextToReadOnly returns new context from the given one, but with
// writing capability turned off, so that you could only invoke Get and Find
// using this new Context. If Context is already read-only this function is a
// no-op. It uses `System.Storage.AsReadOnly` syscall.
func ConvertContextToReadOnly(ctx Context) Context { return Context{} }

// GetContext returns current contract's (that invokes this function) storage
// context. It uses `System.Storage.GetContext` syscall.
func GetContext() Context { return Context{} }

// GetReadOnlyContext returns current contract's (that invokes this function)
// storage context in read-only mode, you can use this context for Get and Find
// functions, but using it for Put and Delete will fail. It uses
// `System.Storage.GetReadOnlyContext` syscall.
func GetReadOnlyContext() Context { return Context{} }

// Put saves given value with given key in the storage using given Context.
// Even though it accepts interface{} for both, you can only pass simple types
// there like string, []byte, int or bool (not structures or slices of more
// complex types). To put more complex types there serialize them first using
// runtime.Serialize. This function uses `System.Storage.Put` syscall.
func Put(ctx Context, key interface{}, value interface{}) {}

// PutEx is an advanced version of Put which saves given value with given key
// and given ReadOnly flag in the storage using given Context. `flag` argument
// can either be odd for constant storage items or even for variable storage items.
// Refer to Put function description for details on how to pass the remaining
// arguments. This function uses `System.Storage.PutEx` syscall.
func PutEx(ctx Context, key interface{}, value interface{}, flag int64) {}

// Get retrieves value stored for the given key using given Context. See Put
// documentation on possible key and value types. If the value is not present in
// the database it returns nil. This function uses `System.Storage.Get` syscall.
func Get(ctx Context, key interface{}) interface{} { return nil }

// Delete removes key-value pair from storage by the given key using given
// Context. See Put documentation on possible key types. This function uses
// `System.Storage.Delete` syscall.
func Delete(ctx Context, key interface{}) {}

// Find returns an iterator.Iterator over key-value pairs in the given Context
// that match the given key (contain it as a prefix). See Put documentation on
// possible key types and iterator package documentation on how to use the
// returned value. This function uses `System.Storage.Find` syscall.
func Find(ctx Context, key interface{}, options FindFlags) iterator.Iterator {
	return iterator.Iterator{}
}
