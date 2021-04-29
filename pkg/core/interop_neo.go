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
	if opts&storage.FindKeysOnly != 0 &&
		opts&(storage.FindDeserialize|storage.FindPick0|storage.FindPick1) != 0 {
		return fmt.Errorf("%w KeysOnly conflicts with other options", errFindInvalidOptions)
	}
	if opts&storage.FindValuesOnly != 0 &&
		opts&(storage.FindKeysOnly|storage.FindRemovePrefix) != 0 {
		return fmt.Errorf("%w: KeysOnly conflicts with ValuesOnly", errFindInvalidOptions)
	}
	if opts&storage.FindPick0 != 0 && opts&storage.FindPick1 != 0 {
		return fmt.Errorf("%w: Pick0 conflicts with Pick1", errFindInvalidOptions)
	}
	if opts&storage.FindDeserialize == 0 && (opts&storage.FindPick0 != 0 || opts&storage.FindPick1 != 0) {
		return fmt.Errorf("%w: PickN is specified without Deserialize", errFindInvalidOptions)
	}
	siMap, err := ic.DAO.GetStorageItemsWithPrefix(stc.ID, prefix)
	if err != nil {
		return err
	}

	filteredMap := stackitem.NewMap()
	for k, v := range siMap {
		key := append(prefix, []byte(k)...)
		keycopy := make([]byte, len(key))
		copy(keycopy, key)
		filteredMap.Add(stackitem.NewByteArray(keycopy), stackitem.NewByteArray(v))
	}
	sort.Slice(filteredMap.Value().([]stackitem.MapElement), func(i, j int) bool {
		return bytes.Compare(filteredMap.Value().([]stackitem.MapElement)[i].Key.Value().([]byte),
			filteredMap.Value().([]stackitem.MapElement)[j].Key.Value().([]byte)) == -1
	})

	item := storage.NewIterator(filteredMap, len(prefix), opts)
	ic.VM.Estack().PushVal(stackitem.NewInterop(item))

	return nil
}
