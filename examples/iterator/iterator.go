package iterator_contract

import (
	"github.com/CityOfZion/neo-storm/interop/iterator"
	"github.com/CityOfZion/neo-storm/interop/runtime"
	"github.com/CityOfZion/neo-storm/interop/storage"
)

func Main() bool {
	iter := storage.Find(storage.GetContext(), []byte("foo"))
	values := iterator.Values(iter)
	keys := iterator.Keys(iter)

	runtime.Notify("found storage values", values)
	runtime.Notify("found storage keys", keys)

	return true
}
