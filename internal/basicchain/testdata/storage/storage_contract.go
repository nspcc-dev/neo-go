/*
Package storage contains contract that puts a set of values inside the storage on
deploy. The contract has a single method returning iterator over these values.
The contract is aimed to test iterator sessions RPC API.
*/
package storage

import (
	"github.com/nspcc-dev/neo-go/pkg/interop/iterator"
	"github.com/nspcc-dev/neo-go/pkg/interop/storage"
)

// valuesCount is the amount of stored values.
const valuesCount = 255

// valuesPrefix is the prefix values are stored by.
var valuesPrefix = []byte{0x01}

func _deploy(data interface{}, isUpdate bool) {
	if !isUpdate {
		ctx := storage.GetContext()
		for i := 0; i < valuesCount; i++ {
			key := append(valuesPrefix, byte(i))
			storage.Put(ctx, key, i)
		}
	}

}

// IterateOverValues returns iterator over contract storage values stored during deploy.
func IterateOverValues() iterator.Iterator {
	ctx := storage.GetContext()
	return storage.Find(ctx, valuesPrefix, storage.ValuesOnly)
}
