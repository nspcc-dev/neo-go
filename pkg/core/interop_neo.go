package core

import (
	"bytes"
	"errors"
	"fmt"
	"sort"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/storage"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

var (
	errGasLimitExceeded   = errors.New("gas limit exceeded")
	errFindInvalidOptions = errors.New("invalid Find options")
)

// storageFind finds stored key-value pair.
func storageFind(ic *interop.Context) error {
	stcInterface := ic.VM.Estack().Pop().Value()
	stc, ok := stcInterface.(*StorageContext)
	if !ok {
		return fmt.Errorf("%T is not a StorageContext", stcInterface)
	}
	prefix := ic.VM.Estack().Pop().Bytes()
	opts := ic.VM.Estack().Pop().BigInt().Int64()
	if opts&^storage.FindAll != 0 {
		return fmt.Errorf("%w: unknown flag", errFindInvalidOptions)
	}
	if opts&storage.FindValuesOnly != 0 &&
		opts&(storage.FindKeysOnly|storage.FindRemovePrefix) != 0 {
		return fmt.Errorf("%w: KeysOnly conflicts with ValuesOnly", errFindInvalidOptions)
	}
	siMap, err := ic.DAO.GetStorageItemsWithPrefix(stc.ID, prefix)
	if err != nil {
		return err
	}

	filteredMap := stackitem.NewMap()
	for k, v := range siMap {
		filteredMap.Add(stackitem.NewByteArray(append(prefix, []byte(k)...)), stackitem.NewByteArray(v.Value))
	}
	sort.Slice(filteredMap.Value().([]stackitem.MapElement), func(i, j int) bool {
		return bytes.Compare(filteredMap.Value().([]stackitem.MapElement)[i].Key.Value().([]byte),
			filteredMap.Value().([]stackitem.MapElement)[j].Key.Value().([]byte)) == -1
	})

	item := storage.NewIterator(filteredMap, opts)
	ic.VM.Estack().PushVal(stackitem.NewInterop(item))

	return nil
}
