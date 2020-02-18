package testdata

import "github.com/CityOfZion/neo-go/pkg/interop/storage"

func Main(operation string, args []interface{}) interface{} {
	ctx := storage.GetContext()
	storage.Put(ctx, args[0].([]byte), args[1].([]byte))
	return true
}
