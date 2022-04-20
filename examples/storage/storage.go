package storagecontract

import (
	"github.com/nspcc-dev/neo-go/pkg/interop/iterator"
	"github.com/nspcc-dev/neo-go/pkg/interop/storage"
)

// ctx holds storage context for contract methods
var ctx storage.Context

// defaultKey represents the default key.
var defaultKey = []byte("default")

// init inits storage context before any other contract method is called
func init() {
	ctx = storage.GetContext()
}

// Put puts the value at the key.
func Put(key, value []byte) []byte {
	storage.Put(ctx, key, value)
	return key
}

// PutDefault puts the value to the default key.
func PutDefault(value []byte) []byte {
	storage.Put(ctx, defaultKey, value)
	return defaultKey
}

// Get returns the value at the passed key.
func Get(key []byte) interface{} {
	return storage.Get(ctx, key)
}

// GetDefault returns the value at the default key.
func GetDefault() interface{} {
	return storage.Get(ctx, defaultKey)
}

// Delete deletes the value at the passed key.
func Delete(key []byte) bool {
	storage.Delete(ctx, key)
	return true
}

// Find returns an array of key-value pairs with the key that matches the passed value.
func Find(value []byte) []string {
	iter := storage.Find(ctx, value, storage.None)
	result := []string{}
	for iterator.Next(iter) {
		val := iterator.Value(iter).([]string)
		result = append(result, val[0]+":"+val[1])
	}
	return result
}
