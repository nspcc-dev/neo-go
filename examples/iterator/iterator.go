package iteratorcontract

import (
	"github.com/nspcc-dev/neo-go/pkg/interop/iterator"
	"github.com/nspcc-dev/neo-go/pkg/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/interop/storage"
)

// NotifyKeysAndValues sends notification with `foo` storage keys and values
func NotifyKeysAndValues() bool {
	iter := storage.Find(storage.GetContext(), []byte("foo"))
	for iterator.Next(iter) {
		runtime.Notify("found storage key-value pair", iterator.Value(iter))
	}
	return true
}
