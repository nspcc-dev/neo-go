package sub

import "github.com/nspcc-dev/neo-go/pkg/interop/storage"

var Key = "sub"

func _deploy(data any, isUpdate bool) {
	ctx := storage.GetContext()
	value := "sub create"
	if isUpdate {
		value = "sub update"
	}
	storage.Put(ctx, Key, value)
}
