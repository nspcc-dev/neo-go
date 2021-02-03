package core

import (
	"crypto/elliptic"
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

const (
	// MaxStorageKeyLen is the maximum length of a key for storage items.
	MaxStorageKeyLen = 64
	// MaxStorageValueLen is the maximum length of a value for storage items.
	// It is set to be the maximum value for uint16.
	MaxStorageValueLen = 65535
)

// StorageContext contains storing id and read/write flag, it's used as
// a context for storage manipulation functions.
type StorageContext struct {
	ID       int32
	ReadOnly bool
}

// StorageFlag represents storage flag which denotes whether the stored value is
// a constant.
type StorageFlag byte

const (
	// None is a storage flag for non-constant items.
	None StorageFlag = 0
	// Constant is a storage flag for constant items.
	Constant StorageFlag = 0x01
)

// engineGetScriptContainer returns transaction or block that contains the script
// being run.
func engineGetScriptContainer(ic *interop.Context) error {
	var item stackitem.Item
	switch t := ic.Container.(type) {
	case *transaction.Transaction:
		item = native.TransactionToStackItem(t)
	case *block.Block:
		item = native.BlockToStackItem(t)
	default:
		return errors.New("unknown script container")
	}
	ic.VM.Estack().PushVal(item)
	return nil
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
	ic.VM.AddGas(ic.Chain.GetPolicer().GetStoragePrice())
	key := ic.VM.Estack().Pop().Bytes()
	si := ic.DAO.GetStorageItem(stc.ID, key)
	if si != nil && si.IsConst {
		return errors.New("storage item is constant")
	}
	return ic.DAO.DeleteStorageItem(stc.ID, key)
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
	if si != nil && si.Value != nil {
		ic.VM.Estack().PushVal(si.Value)
	} else {
		ic.VM.Estack().PushVal(stackitem.Null{})
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
	ic.VM.Estack().PushVal(stackitem.NewInterop(sc))
	return nil
}

func putWithContextAndFlags(ic *interop.Context, stc *StorageContext, key []byte, value []byte, isConst bool) error {
	if len(key) > MaxStorageKeyLen {
		return errors.New("key is too big")
	}
	if len(value) > MaxStorageValueLen {
		return errors.New("value is too big")
	}
	if stc.ReadOnly {
		return errors.New("StorageContext is read only")
	}
	si := ic.DAO.GetStorageItem(stc.ID, key)
	if si != nil && si.IsConst {
		return errors.New("storage item exists and is read-only")
	}
	sizeInc := 1
	if si == nil {
		si = &state.StorageItem{}
		sizeInc = len(key) + len(value)
	} else if len(value) != 0 {
		if len(value) <= len(si.Value) {
			sizeInc = (len(value)-1)/4 + 1
		} else {
			sizeInc = (len(si.Value)-1)/4 + 1 + len(value) - len(si.Value)
		}
	}
	if !ic.VM.AddGas(int64(sizeInc) * ic.Chain.GetPolicer().GetStoragePrice()) {
		return errGasLimitExceeded
	}
	si.Value = value
	si.IsConst = isConst
	return ic.DAO.PutStorageItem(stc.ID, key, si)
}

// storagePutInternal is a unified implementation of storagePut and storagePutEx.
func storagePutInternal(ic *interop.Context, getFlag bool) error {
	stcInterface := ic.VM.Estack().Pop().Value()
	stc, ok := stcInterface.(*StorageContext)
	if !ok {
		return fmt.Errorf("%T is not a StorageContext", stcInterface)
	}
	key := ic.VM.Estack().Pop().Bytes()
	value := ic.VM.Estack().Pop().Bytes()
	var flag int
	if getFlag {
		flag = int(ic.VM.Estack().Pop().BigInt().Int64())
	}
	return putWithContextAndFlags(ic, stc, key, value, int(Constant)&flag != 0)
}

// storagePut puts key-value pair into the storage.
func storagePut(ic *interop.Context) error {
	return storagePutInternal(ic, false)
}

// storagePutEx puts key-value pair with given flags into the storage.
func storagePutEx(ic *interop.Context) error {
	return storagePutInternal(ic, true)
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
	ic.VM.Estack().PushVal(stackitem.NewInterop(stc))
	return nil
}

// contractIsStandard checks if contract is standard (sig or multisig) contract.
func contractIsStandard(ic *interop.Context) error {
	h := ic.VM.Estack().Pop().Bytes()
	u, err := util.Uint160DecodeBytesBE(h)
	if err != nil {
		return err
	}
	var result bool
	cs, _ := ic.GetContract(u)
	if cs != nil {
		result = vm.IsStandardContract(cs.NEF.Script)
	} else {
		if tx, ok := ic.Container.(*transaction.Transaction); ok {
			for _, witness := range tx.Scripts {
				if witness.ScriptHash() == u {
					result = vm.IsStandardContract(witness.VerificationScript)
					break
				}
			}
		}
	}
	ic.VM.Estack().PushVal(result)
	return nil
}

// contractCreateStandardAccount calculates contract scripthash for a given public key.
func contractCreateStandardAccount(ic *interop.Context) error {
	h := ic.VM.Estack().Pop().Bytes()
	p, err := keys.NewPublicKeyFromBytes(h, elliptic.P256())
	if err != nil {
		return err
	}
	ic.VM.Estack().PushVal(p.GetScriptHash().BytesBE())
	return nil
}

// contractGetCallFlags returns current context calling flags.
func contractGetCallFlags(ic *interop.Context) error {
	ic.VM.Estack().PushVal(ic.VM.Context().GetCallFlags())
	return nil
}
