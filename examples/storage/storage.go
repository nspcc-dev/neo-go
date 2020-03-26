package storagecontract

import (
	"github.com/nspcc-dev/neo-go/pkg/interop/iterator"
	"github.com/nspcc-dev/neo-go/pkg/interop/storage"
)

// Main is a very useful function.
func Main(operation string, args []interface{}) interface{} {
	ctx := storage.GetContext()

	// Puts value at key
	if operation == "put" {
		if checkArgs(args, 2) {
			key := args[0].([]byte)
			value := args[1].([]byte)
			storage.Put(ctx, key, value)
			return key
		}
	}

	// Returns the value at passed key
	if operation == "get" {
		if checkArgs(args, 1) {
			key := args[0].([]byte)
			return storage.Get(ctx, key)
		}
	}

	// Deletes the value at passed key
	if operation == "delete" {
		key := args[0].([]byte)
		storage.Delete(ctx, key)
		return true
	}

	// Returns an array of key-value pairs with key that matched the passed value
	if operation == "find" {
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
	}

	return false
}

func checkArgs(args []interface{}, length int) bool {
	if len(args) == length {
		return true
	}

	return false
}
