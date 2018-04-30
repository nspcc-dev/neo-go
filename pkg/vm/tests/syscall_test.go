package vm_test

import (
	"testing"
)

func TestStoragePutGet(t *testing.T) {
	src := `
		package foo

		import "github.com/CityOfZion/neo-go/pkg/vm/api/storage"

		func Main() string {
			ctx := storage.GetContext()
			key := []byte("token")
			storage.Put(ctx, key, []byte("foo"))
			x := storage.Get(ctx, key)
			return x.(string)
		}
	`
	eval(t, src, []byte("foo"))
}
