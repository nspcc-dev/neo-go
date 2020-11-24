package iteratorcontract

import (
	"github.com/nspcc-dev/neo-go/pkg/interop/iterator"
	"github.com/nspcc-dev/neo-go/pkg/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/interop/storage"
)

// NotifyKeysAndValues sends notification with `foo` storage keys and values
func NotifyKeysAndValues() bool {
	iter := storage.Find(storage.GetContext(), []byte("foo"))
	values := iterator.Values(iter)
	keys := iterator.Keys(iter)

	runtime.Notify("found storage values", values)
	// For illustration purposes event is emitted with 'Any' type.
	var typedKeys interface{} = keys
	runtime.Notify("found storage keys", typedKeys)

	return true
}
