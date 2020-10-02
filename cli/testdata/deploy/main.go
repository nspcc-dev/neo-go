package deploy

import (
	"github.com/nspcc-dev/neo-go/cli/testdata/deploy/sub"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/interop/storage"
)

var key = "key"

func _deploy(isUpdate bool) {
	ctx := storage.GetContext()
	value := "on create"
	if isUpdate {
		value = "on update"
	}
	storage.Put(ctx, key, value)
}

// Update updates contract with the new one.
func Update(script, manifest []byte) {
	contract.Update(script, manifest)
}

// GetValue returns stored value.
func GetValue() string {
	ctx := storage.GetReadOnlyContext()
	val1 := storage.Get(ctx, key)
	val2 := storage.Get(ctx, sub.Key)
	return val1.(string) + "|" + val2.(string)
}
