package storage

import (
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/config/limits"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

var (
	// ErrGasLimitExceeded is returned from interops when there is not enough
	// GAS left in the execution context to complete the action.
	ErrGasLimitExceeded   = errors.New("gas limit exceeded")
	errFindInvalidOptions = errors.New("invalid Find options")
)

// Context contains contract ID and read/write flag, it's used as
// a context for storage manipulation functions.
type Context struct {
	ID       int32
	ReadOnly bool
}

func deleteWithContext(ic *interop.Context, stc *Context) {
	key := ic.VM.Estack().Pop().Bytes()
	ic.DAO.DeleteStorageItem(stc.ID, key)
}

// storageDelete deletes stored key-value pair.
func Delete(ic *interop.Context) error {
	stcInterface := ic.VM.Estack().Pop().Value()
	stc, ok := stcInterface.(*Context)
	if !ok {
		return fmt.Errorf("%T is not a storage.Context", stcInterface)
	}
	if stc.ReadOnly {
		return errors.New("storage.Context is read only")
	}
	deleteWithContext(ic, stc)
	return nil
}

// LocalDelete is similar to Delete, but does not require storage context.
func LocalDelete(ic *interop.Context) error {
	contract, err := ic.GetContract(ic.VM.GetCurrentScriptHash())
	if err != nil {
		return fmt.Errorf("storage context can not be retrieved in dynamic scripts: %w", err)
	}
	deleteWithContext(ic, &Context{
		ID: contract.ID,
	})
	return nil
}

func getWithContext(ic *interop.Context, stc *Context) {
	key := ic.VM.Estack().Pop().Bytes()
	si := ic.DAO.GetStorageItem(stc.ID, key)
	if si != nil {
		ic.VM.Estack().PushItem(stackitem.NewByteArray([]byte(si)))
	} else {
		ic.VM.Estack().PushItem(stackitem.Null{})
	}
}

// Get returns stored key-value pair.
func Get(ic *interop.Context) error {
	stcInterface := ic.VM.Estack().Pop().Value()
	stc, ok := stcInterface.(*Context)
	if !ok {
		return fmt.Errorf("%T is not a storage.Context", stcInterface)
	}
	getWithContext(ic, stc)
	return nil
}

// LocalGet is similar to Get, but does not require storage context.
func LocalGet(ic *interop.Context) error {
	contract, err := ic.GetContract(ic.VM.GetCurrentScriptHash())
	if err != nil {
		return fmt.Errorf("storage context can not be retrieved in dynamic scripts: %w", err)
	}
	getWithContext(ic, &Context{
		ID:       contract.ID,
		ReadOnly: true,
	})
	return nil
}

// GetContext returns storage context for the currently executing contract.
func GetContext(ic *interop.Context) error {
	return getContextInternal(ic, false)
}

// GetReadOnlyContext returns read-only storage context for the currently executing contract.
func GetReadOnlyContext(ic *interop.Context) error {
	return getContextInternal(ic, true)
}

// getContextInternal is internal version of storageGetContext and
// storageGetReadOnlyContext which allows to specify ReadOnly context flag.
func getContextInternal(ic *interop.Context, isReadOnly bool) error {
	contract, err := ic.GetContract(ic.VM.GetCurrentScriptHash())
	if err != nil {
		return fmt.Errorf("storage context can not be retrieved in dynamic scripts: %w", err)
	}
	sc := &Context{
		ID:       contract.ID,
		ReadOnly: isReadOnly,
	}
	ic.VM.Estack().PushItem(stackitem.NewInterop(sc))
	return nil
}

func putWithContext(ic *interop.Context, stc *Context, key []byte, value []byte) error {
	if len(key) > limits.MaxStorageKeyLen {
		return errors.New("key is too big")
	}
	if len(value) > limits.MaxStorageValueLen {
		return errors.New("value is too big")
	}
	if stc.ReadOnly {
		return errors.New("storage.Context is read only")
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
		return ErrGasLimitExceeded
	}
	ic.DAO.PutStorageItem(stc.ID, key, value)
	return nil
}

// Put puts key-value pair into the storage.
func Put(ic *interop.Context) error {
	stcInterface := ic.VM.Estack().Pop().Value()
	stc, ok := stcInterface.(*Context)
	if !ok {
		return fmt.Errorf("%T is not a storage.Context", stcInterface)
	}
	key := ic.VM.Estack().Pop().Bytes()
	value := ic.VM.Estack().Pop().Bytes()
	return putWithContext(ic, stc, key, value)
}

// LocalPut is similar to Put, but does not require storage context.
func LocalPut(ic *interop.Context) error {
	contract, err := ic.GetContract(ic.VM.GetCurrentScriptHash())
	if err != nil {
		return fmt.Errorf("storage context can not be retrieved in dynamic scripts: %w", err)
	}
	key := ic.VM.Estack().Pop().Bytes()
	value := ic.VM.Estack().Pop().Bytes()
	return putWithContext(ic, &Context{
		ID: contract.ID,
	}, key, value)
}

// ContextAsReadOnly sets given context to read-only mode.
func ContextAsReadOnly(ic *interop.Context) error {
	stcInterface := ic.VM.Estack().Pop().Value()
	stc, ok := stcInterface.(*Context)
	if !ok {
		return fmt.Errorf("%T is not a storage.Context", stcInterface)
	}
	if !stc.ReadOnly {
		stx := &Context{
			ID:       stc.ID,
			ReadOnly: true,
		}
		stc = stx
	}
	ic.VM.Estack().PushItem(stackitem.NewInterop(stc))
	return nil
}
