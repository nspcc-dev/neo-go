/*
Package storage provides functions to access and modify contract's storage.
Neo storage's model follows simple key-value DB pattern, this storage is a part
of blockchain state, so you can use it between various invocations of the same
contract.
*/
package storage

import (
	"github.com/nspcc-dev/neo-go/pkg/interop/iterator"
	"github.com/nspcc-dev/neo-go/pkg/interop/neogointernal"
)

// Context represents storage context that is mandatory for Put/Get/Delete
// operations. It's an opaque type that can only be created properly by
// GetContext, GetReadOnlyContext or ConvertContextToReadOnly. It's similar
// to Neo .net framework's StorageContext class.
type Context struct{}

// FindFlags represents parameters to `Find` iterator.
type FindFlags byte

// KeyValue represents a key-value pair that may be used to cast values returned
// by [iterator.Value] method for cases when it returns key-value pairs.
type KeyValue struct {
	Key   []byte
	Value []byte
}

const (
	// None is default option. Iterator values are key-value pairs.
	None FindFlags = 0
	// KeysOnly is used for iterating over keys.
	KeysOnly FindFlags = 1 << 0
	// RemovePrefix is used for stripping prefix (passed to Find) from keys.
	RemovePrefix FindFlags = 1 << 1
	// ValuesOnly is used for iterating over values.
	ValuesOnly FindFlags = 1 << 2
	// DeserializeValues is used for deserializing values on-the-fly.
	// It can be combined with other options.
	DeserializeValues FindFlags = 1 << 3
	// PickField0 is used to get first field in a serialized struct or array.
	PickField0 FindFlags = 1 << 4
	// PickField1 is used to get second field in a serialized struct or array.
	PickField1 FindFlags = 1 << 5
	// Backwards is used to iterate over elements in reversed (descending) order.
	Backwards FindFlags = 1 << 7
)

// ConvertContextToReadOnly returns new context from the given one, but with
// writing capability turned off, so that you could only invoke Get and Find
// using this new Context. If Context is already read-only this function is a
// no-op. It uses `System.Storage.AsReadOnly` syscall.
func ConvertContextToReadOnly(ctx Context) Context {
	return neogointernal.Syscall1("System.Storage.AsReadOnly", ctx).(Context)
}

// GetContext returns current contract's (that invokes this function) storage
// context. It uses `System.Storage.GetContext` syscall.
func GetContext() Context {
	return neogointernal.Syscall0("System.Storage.GetContext").(Context)
}

// GetReadOnlyContext returns current contract's (that invokes this function)
// storage context in read-only mode, you can use this context for Get and Find
// functions, but using it for Put and Delete will fail. It uses
// `System.Storage.GetReadOnlyContext` syscall.
func GetReadOnlyContext() Context {
	return neogointernal.Syscall0("System.Storage.GetReadOnlyContext").(Context)
}

// Put saves given value with given key in the storage using given Context.
// Even though it accepts interface{} hidden under `any` for both, you can only
// pass simple types there like string, []byte, int or bool (not structures or
// slices of more complex types). To put more complex types there serialize them
// first using runtime.Serialize. This function uses `System.Storage.Put` syscall.
func Put(ctx Context, key any, value any) {
	neogointernal.Syscall3NoReturn("System.Storage.Put", ctx, key, value)
}

// LocalPut is similar to Put, but does not require context.
func LocalPut(key []byte, value []byte) {
	neogointernal.Syscall2NoReturn("System.Storage.Local.Put", key, value)
}

// Get retrieves value stored for the given key using given Context. See Put
// documentation on possible key and value types. If the value is not present in
// the database it returns nil. This function uses `System.Storage.Get` syscall.
func Get(ctx Context, key any) any {
	return neogointernal.Syscall2("System.Storage.Get", ctx, key)
}

// LocalGet is similar to Get, but does not require context.
func LocalGet(key []byte) any {
	return neogointernal.Syscall1("System.Storage.Local.Get", key)
}

// Delete removes key-value pair from storage by the given key using given
// Context. See Put documentation on possible key types. This function uses
// `System.Storage.Delete` syscall.
func Delete(ctx Context, key any) {
	neogointernal.Syscall2NoReturn("System.Storage.Delete", ctx, key)
}

// LocalDelete is similar to Delete, but does not require context.
func LocalDelete(key []byte) {
	neogointernal.Syscall1NoReturn("System.Storage.Local.Delete", key)
}

// Find returns an iterator.Iterator over key-value pairs in the given Context
// that match the given key (contain it as a prefix). See Put documentation on
// possible key types and iterator package documentation on how to use the
// returned value. This function uses `System.Storage.Find` syscall.
func Find(ctx Context, key any, options FindFlags) iterator.Iterator {
	return neogointernal.Syscall3("System.Storage.Find", ctx, key, options).(iterator.Iterator)
}

// LocalFind is similar to Find, but does not require context.
func LocalFind(key []byte, options FindFlags) iterator.Iterator {
	return neogointernal.Syscall2("System.Storage.Local.Find", key, options).(iterator.Iterator)
}
