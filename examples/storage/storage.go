package storagecontract

import (
	"github.com/nspcc-dev/neo-go/pkg/interop/iterator"
	"github.com/nspcc-dev/neo-go/pkg/interop/storage"
)

// ctx holds storage context for contract methods
var ctx storage.Context

// init inits storage context before any other contract method is called
func init() {
	ctx = storage.GetContext()
}

// Put puts value at key.
func Put(key, value []byte) []byte {
	storage.Put(ctx, key, value)
	return key
}

// Get returns the value at passed key.
func Get(key []byte) interface{} {
	return storage.Get(ctx, key)
}

// Delete deletes the value at passed key.
func Delete(key []byte) bool {
	storage.Delete(ctx, key)
	return true
}

// Find returns an array of key-value pairs with key that matched the passed value
func Find(value []byte) []string {
	iter := storage.Find(ctx, value, storage.None)
	result := []string{}
	for iterator.Next(iter) {
		val := iterator.Value(iter).([]string)
		result = append(result, val[0]+":"+val[1])
	}
	return result
}
