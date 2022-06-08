package core

import (
	"context"
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	istorage "github.com/nspcc-dev/neo-go/pkg/core/interop/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

var (
	errGasLimitExceeded   = errors.New("gas limit exceeded")
	errFindInvalidOptions = errors.New("invalid Find options")
)

// StorageContext contains storing id and read/write flag, it's used as
// a context for storage manipulation functions.
type StorageContext struct {
	ID       int32
	ReadOnly bool
}

// storageDelete deletes stored key-value pair.
func storageDelete(ic *interop.Context) error {
	stcInterface := ic.VM.Estack().Pop().Value()
	stc, ok := stcInterface.(*StorageContext)
	if !ok {
		return fmt.Errorf("%T is not a StorageContext", stcInterface)
	}
	if stc.ReadOnly {
		return errors.New("StorageContext is read only")
	}
	key := ic.VM.Estack().Pop().Bytes()
	ic.DAO.DeleteStorageItem(stc.ID, key)
	return nil
}

// storageGet returns stored key-value pair.
func storageGet(ic *interop.Context) error {
	stcInterface := ic.VM.Estack().Pop().Value()
	stc, ok := stcInterface.(*StorageContext)
	if !ok {
		return fmt.Errorf("%T is not a StorageContext", stcInterface)
	}
	key := ic.VM.Estack().Pop().Bytes()
	si := ic.DAO.GetStorageItem(stc.ID, key)
	if si != nil {
		ic.VM.Estack().PushItem(stackitem.NewByteArray([]byte(si)))
	} else {
		ic.VM.Estack().PushItem(stackitem.Null{})
	}
	return nil
}

// storageGetContext returns storage context (scripthash).
func storageGetContext(ic *interop.Context) error {
	return storageGetContextInternal(ic, false)
}

// storageGetReadOnlyContext returns read-only context (scripthash).
func storageGetReadOnlyContext(ic *interop.Context) error {
	return storageGetContextInternal(ic, true)
}

// storageGetContextInternal is internal version of storageGetContext and
// storageGetReadOnlyContext which allows to specify ReadOnly context flag.
func storageGetContextInternal(ic *interop.Context, isReadOnly bool) error {
	contract, err := ic.GetContract(ic.VM.GetCurrentScriptHash())
	if err != nil {
		return err
	}
	sc := &StorageContext{
		ID:       contract.ID,
		ReadOnly: isReadOnly,
	}
	ic.VM.Estack().PushItem(stackitem.NewInterop(sc))
	return nil
}

func putWithContext(ic *interop.Context, stc *StorageContext, key []byte, value []byte) error {
	if len(key) > storage.MaxStorageKeyLen {
		return errors.New("key is too big")
	}
	if len(value) > storage.MaxStorageValueLen {
		return errors.New("value is too big")
	}
	if stc.ReadOnly {
		return errors.New("StorageContext is read only")
	}
	si := ic.DAO.GetStorageItem(stc.ID, key)
	sizeInc := len(value)
	if si == nil {
		sizeInc = len(key) + len(value)
	} else if len(value) != 0 {
		if len(value) <= len(si) {
			sizeInc = (len(value)-1)/4 + 1
		} else if len(si) != 0 {
			sizeInc = (len(si)-1)/4 + 1 + len(value) - len(si)
		}
	}
	if !ic.VM.AddGas(int64(sizeInc) * ic.BaseStorageFee()) {
		return errGasLimitExceeded
	}
	ic.DAO.PutStorageItem(stc.ID, key, value)
	return nil
}

// storagePut puts key-value pair into the storage.
func storagePut(ic *interop.Context) error {
	stcInterface := ic.VM.Estack().Pop().Value()
	stc, ok := stcInterface.(*StorageContext)
	if !ok {
		return fmt.Errorf("%T is not a StorageContext", stcInterface)
	}
	key := ic.VM.Estack().Pop().Bytes()
	value := ic.VM.Estack().Pop().Bytes()
	return putWithContext(ic, stc, key, value)
}

// storageContextAsReadOnly sets given context to read-only mode.
func storageContextAsReadOnly(ic *interop.Context) error {
	stcInterface := ic.VM.Estack().Pop().Value()
	stc, ok := stcInterface.(*StorageContext)
	if !ok {
		return fmt.Errorf("%T is not a StorageContext", stcInterface)
	}
	if !stc.ReadOnly {
		stx := &StorageContext{
			ID:       stc.ID,
			ReadOnly: true,
		}
		stc = stx
	}
	ic.VM.Estack().PushItem(stackitem.NewInterop(stc))
	return nil
}

// storageFind finds stored key-value pair.
func storageFind(ic *interop.Context) error {
	stcInterface := ic.VM.Estack().Pop().Value()
	stc, ok := stcInterface.(*StorageContext)
	if !ok {
		return fmt.Errorf("%T is not a StorageContext", stcInterface)
	}
	prefix := ic.VM.Estack().Pop().Bytes()
	opts := ic.VM.Estack().Pop().BigInt().Int64()
	if opts&^istorage.FindAll != 0 {
		return fmt.Errorf("%w: unknown flag", errFindInvalidOptions)
	}
	if opts&istorage.FindKeysOnly != 0 &&
		opts&(istorage.FindDeserialize|istorage.FindPick0|istorage.FindPick1) != 0 {
		return fmt.Errorf("%w KeysOnly conflicts with other options", errFindInvalidOptions)
	}
	if opts&istorage.FindValuesOnly != 0 &&
		opts&(istorage.FindKeysOnly|istorage.FindRemovePrefix) != 0 {
		return fmt.Errorf("%w: KeysOnly conflicts with ValuesOnly", errFindInvalidOptions)
	}
	if opts&istorage.FindPick0 != 0 && opts&istorage.FindPick1 != 0 {
		return fmt.Errorf("%w: Pick0 conflicts with Pick1", errFindInvalidOptions)
	}
	if opts&istorage.FindDeserialize == 0 && (opts&istorage.FindPick0 != 0 || opts&istorage.FindPick1 != 0) {
		return fmt.Errorf("%w: PickN is specified without Deserialize", errFindInvalidOptions)
	}
	ctx, cancel := context.WithCancel(context.Background())
	seekres := ic.DAO.SeekAsync(ctx, stc.ID, storage.SeekRange{Prefix: prefix})
	item := istorage.NewIterator(seekres, prefix, opts)
	ic.VM.Estack().PushItem(stackitem.NewInterop(item))
	ic.RegisterCancelFunc(func() {
		cancel()
		// Underlying persistent store is likely to be a private MemCachedStore. Thus,
		// to avoid concurrent map iteration and map write we need to wait until internal
		// seek goroutine is finished, because it can access underlying persistent store.
		for range seekres {
		}
	})

	return nil
}
