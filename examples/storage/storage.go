package storagecontract

import (
	"github.com/nspcc-dev/neo-go/pkg/interop/iterator"
	"github.com/nspcc-dev/neo-go/pkg/interop/storage"
)

// Main is a very useful function.
func Main(operation string, args []interface{}) interface{} {
	if operation == "put" {
		return Put(args)
	}

	if operation == "get" {
		return Get(args)
	}

	if operation == "delete" {
		return Delete(args)
	}

	if operation == "find" {
		return Find(args)
	}

	return false
}

// Put puts value at key.
func Put(args []interface{}) interface{} {
	ctx := storage.GetContext()
	if checkArgs(args, 2) {
		key := args[0].([]byte)
		value := args[1].([]byte)
		storage.Put(ctx, key, value)
		return key
	}
	return false
}

// Get returns the value at passed key.
func Get(args []interface{}) interface{} {
	ctx := storage.GetContext()
	if checkArgs(args, 1) {
		key := args[0].([]byte)
		return storage.Get(ctx, key)
	}
	return false
}

// Delete deletes the value at passed key.
func Delete(args []interface{}) interface{} {
	ctx := storage.GetContext()
	key := args[0].([]byte)
	storage.Delete(ctx, key)
	return true
}

// Find returns an array of key-value pairs with key that matched the passed value.
func Find(args []interface{}) interface{} {
	ctx := storage.GetContext()
	if checkArgs(args, 1) {
		value := args[0].([]byte)
		iter := storage.Find(ctx, value)
		result := []string{}
		for iterator.Next(iter) {
			val := iterator.Value(iter)
			key := iterator.Key(iter)
			result = append(result, key.(string)+":"+val.(string))
		}
		return result
	}
	return false
}

func checkArgs(args []interface{}, length int) bool {
	if len(args) == length {
		return true
	}

	return false
}
