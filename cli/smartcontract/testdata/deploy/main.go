package deploy

import (
	"github.com/nspcc-dev/neo-go/cli/smartcontract/testdata/deploy/sub"
	"github.com/nspcc-dev/neo-go/pkg/interop"
	"github.com/nspcc-dev/neo-go/pkg/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/interop/iterator"
	"github.com/nspcc-dev/neo-go/pkg/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/interop/storage"
)

var key = "key"

const mgmtKey = "mgmt"

func _deploy(data any, isUpdate bool) {
	var value string

	ctx := storage.GetContext()
	if isUpdate {
		value = "on update"
	} else {
		value = "on create"
		sh := runtime.GetCallingScriptHash()
		storage.Put(ctx, mgmtKey, sh)

		if data != nil {
			arr := data.([]any)
			for i := 0; i < len(arr)-1; i += 2 {
				storage.Put(ctx, arr[i], arr[i+1])
			}
		}
	}

	storage.Put(ctx, key, value)
}

// Fail just fails.
func Fail() {
	panic("as expected")
}

// CheckSenderWitness checks sender's witness.
func CheckSenderWitness() {
	tx := runtime.GetScriptContainer()
	if !runtime.CheckWitness(tx.Sender) {
		panic("not witnessed")
	}
}

// Update updates the contract with a new one.
func Update(script, manifest []byte) {
	ctx := storage.GetReadOnlyContext()
	mgmt := storage.Get(ctx, mgmtKey).(interop.Hash160)
	contract.Call(mgmt, "update", contract.All, script, manifest)
}

// GetValue returns the stored value.
func GetValue() string {
	ctx := storage.GetReadOnlyContext()
	val1 := storage.Get(ctx, key)
	val2 := storage.Get(ctx, sub.Key)
	return val1.(string) + "|" + val2.(string)
}

// GetValueWithKey returns the stored value with the specified key.
func GetValueWithKey(key string) string {
	ctx := storage.GetReadOnlyContext()
	return storage.Get(ctx, key).(string)
}

// TestFind finds items with the specified prefix.
func TestFind(f storage.FindFlags) []any {
	ctx := storage.GetContext()
	storage.Put(ctx, "findkey1", "value1")
	storage.Put(ctx, "findkey2", "value2")

	var result []any
	iter := storage.Find(ctx, "findkey", f)
	for iterator.Next(iter) {
		result = append(result, iterator.Value(iter))
	}
	return result
}
