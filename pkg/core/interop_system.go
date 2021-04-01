package core

import (
	"crypto/elliptic"
	"errors"
	"fmt"
	"math"

	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// StorageContext contains storing id and read/write flag, it's used as
// a context for storage manipulation functions.
type StorageContext struct {
	ID       int32
	ReadOnly bool
}

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
	key := ic.VM.Estack().Pop().Bytes()
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
	if si != nil {
		ic.VM.Estack().PushVal([]byte(si))
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
	if !ic.VM.AddGas(int64(sizeInc) * ic.Chain.GetPolicer().GetStoragePrice()) {
		return errGasLimitExceeded
	}
	return ic.DAO.PutStorageItem(stc.ID, key, value)
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
	ic.VM.Estack().PushVal(stackitem.NewInterop(stc))
	return nil
}

// contractCreateMultisigAccount calculates multisig contract scripthash for a
// given m and a set of public keys.
func contractCreateMultisigAccount(ic *interop.Context) error {
	m := ic.VM.Estack().Pop().BigInt()
	if !m.IsInt64() || m.Int64() > math.MaxInt32 {
		return errors.New("m should fit int32")
	}
	arr := ic.VM.Estack().Pop().Array()
	pubs := make(keys.PublicKeys, len(arr))
	for i, pk := range arr {
		p, err := keys.NewPublicKeyFromBytes(pk.Value().([]byte), elliptic.P256())
		if err != nil {
			return err
		}
		pubs[i] = p
	}
	script, err := smartcontract.CreateMultiSigRedeemScript(int(m.Int64()), pubs)
	if err != nil {
		return err
	}
	ic.VM.Estack().PushVal(hash.Hash160(script).BytesBE())
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
