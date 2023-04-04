package iteratorcontract

import (
	"github.com/nspcc-dev/neo-go/pkg/interop/iterator"
	"github.com/nspcc-dev/neo-go/pkg/interop/runtime"
	"github.com/nspcc-dev/neo-go/pkg/interop/storage"
)

// _deploy primes contract's storage with some data to be used later.
func _deploy(_ any, _ bool) {
	ctx := storage.GetContext() // RW context.
	storage.Put(ctx, "foo1", "1")
	storage.Put(ctx, "foo2", "2")
	storage.Put(ctx, "foo3", "3")
}

// NotifyKeysAndValues sends notification with `foo` storage keys and values.
func NotifyKeysAndValues() bool {
	iter := storage.Find(storage.GetReadOnlyContext(), []byte("foo"), storage.None)
	for iterator.Next(iter) {
		runtime.Notify("Key-Value", iterator.Value(iter))
	}
	return true
}

// NotifyValues sends notification with `foo` storage values.
func NotifyValues() bool {
	iter := storage.Find(storage.GetReadOnlyContext(), []byte("foo"), storage.ValuesOnly)
	for iterator.Next(iter) {
		runtime.Notify("Value", iterator.Value(iter))
	}
	return true
}
