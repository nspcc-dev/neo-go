package storage_contract

import (
	"github.com/CityOfZion/neo-storm/interop/storage"
)

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

	// TODO: storage.Find()

	return false
}

func checkArgs(args []interface{}, length int) bool {
	if len(args) == length {
		return true
	}

	return false
}
