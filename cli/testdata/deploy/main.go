package deploy

import (
	"github.com/nspcc-dev/neo-go/cli/testdata/deploy/sub"
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/interop/storage"
)

var key = "key"

const mgmtKey = "mgmt"

func _deploy(isUpdate bool) {
	var value string

	ctx := storage.GetContext()
	if isUpdate {
		value = "on update"
	} else {
		value = "on create"
		sh := runtime.GetCallingScriptHash()
		storage.Put(ctx, mgmtKey, sh)
	}

	storage.Put(ctx, key, value)
}

// Fail just fails.
func Fail() {
	panic("as expected")
}

// Update updates contract with the new one.
func Update(script, manifest []byte) {
	ctx := storage.GetReadOnlyContext()
	mgmt := storage.Get(ctx, mgmtKey).(interop.Hash160)
	contract.Call(mgmt, "update", contract.All, script, manifest)
}

// GetValue returns stored value.
func GetValue() string {
	ctx := storage.GetReadOnlyContext()
	val1 := storage.Get(ctx, key)
	val2 := storage.Get(ctx, sub.Key)
	return val1.(string) + "|" + val2.(string)
}
