package core

import (
	"errors"
	"fmt"
	"math"

	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	gherr "github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	// MaxStorageKeyLen is the maximum length of a key for storage items.
	MaxStorageKeyLen = 1024
)

// StorageContext contains storing script hash and read/write flag, it's used as
// a context for storage manipulation functions.
type StorageContext struct {
	ScriptHash util.Uint160
	ReadOnly   bool
}

// getBlockHashFromElement converts given vm.Element to block hash using given
// Blockchainer if needed. Interop functions accept both block numbers and
// block hashes as parameters, thus this function is needed.
func getBlockHashFromElement(bc Blockchainer, element *vm.Element) (util.Uint256, error) {
	var hash util.Uint256
	hashbytes := element.Bytes()
	if len(hashbytes) <= 5 {
		hashint := element.BigInt().Int64()
		if hashint < 0 || hashint > math.MaxUint32 {
			return hash, errors.New("bad block index")
		}
		hash = bc.GetHeaderHash(int(hashint))
	} else {
		return util.Uint256DecodeBytesBE(hashbytes)
	}
	return hash, nil
}

// bcGetBlock returns current block.
func (ic *interopContext) bcGetBlock(v *vm.VM) error {
	hash, err := getBlockHashFromElement(ic.bc, v.Estack().Pop())
	if err != nil {
		return err
	}
	block, err := ic.bc.GetBlock(hash)
	if err != nil {
		v.Estack().PushVal([]byte{})
	} else {
		v.Estack().PushVal(vm.NewInteropItem(block))
	}
	return nil
}

// bcGetContract returns contract.
func (ic *interopContext) bcGetContract(v *vm.VM) error {
	hashbytes := v.Estack().Pop().Bytes()
	hash, err := util.Uint160DecodeBytesBE(hashbytes)
	if err != nil {
		return err
	}
	cs, err := ic.dao.GetContractState(hash)
	if err != nil {
		v.Estack().PushVal([]byte{})
	} else {
		v.Estack().PushVal(vm.NewInteropItem(cs))
	}
	return nil
}

// bcGetHeader returns block header.
func (ic *interopContext) bcGetHeader(v *vm.VM) error {
	hash, err := getBlockHashFromElement(ic.bc, v.Estack().Pop())
	if err != nil {
		return err
	}
	header, err := ic.bc.GetHeader(hash)
	if err != nil {
		v.Estack().PushVal([]byte{})
	} else {
		v.Estack().PushVal(vm.NewInteropItem(header))
	}
	return nil
}

// bcGetHeight returns blockchain height.
func (ic *interopContext) bcGetHeight(v *vm.VM) error {
	v.Estack().PushVal(ic.bc.BlockHeight())
	return nil
}

// getTransactionAndHeight gets parameter from the vm evaluation stack and
// returns transaction and its height if it's present in the blockchain.
func getTransactionAndHeight(cd *cachedDao, v *vm.VM) (*transaction.Transaction, uint32, error) {
	hashbytes := v.Estack().Pop().Bytes()
	hash, err := util.Uint256DecodeBytesBE(hashbytes)
	if err != nil {
		return nil, 0, err
	}
	return cd.GetTransaction(hash)
}

// bcGetTransaction returns transaction.
func (ic *interopContext) bcGetTransaction(v *vm.VM) error {
	tx, _, err := getTransactionAndHeight(ic.dao, v)
	if err != nil {
		return err
	}
	v.Estack().PushVal(vm.NewInteropItem(tx))
	return nil
}

// bcGetTransactionHeight returns transaction height.
func (ic *interopContext) bcGetTransactionHeight(v *vm.VM) error {
	_, h, err := getTransactionAndHeight(ic.dao, v)
	if err != nil {
		return err
	}
	v.Estack().PushVal(h)
	return nil
}

// popHeaderFromVM returns pointer to Header or error. It's main feature is
// proper treatment of Block structure, because C# code implicitly assumes
// that header APIs can also operate on blocks.
func popHeaderFromVM(v *vm.VM) (*block.Header, error) {
	iface := v.Estack().Pop().Value()
	header, ok := iface.(*block.Header)
	if !ok {
		block, ok := iface.(*block.Block)
		if !ok {
			return nil, errors.New("value is not a header or block")
		}
		return block.Header(), nil
	}
	return header, nil
}

// headerGetIndex returns block index from the header.
func (ic *interopContext) headerGetIndex(v *vm.VM) error {
	header, err := popHeaderFromVM(v)
	if err != nil {
		return err
	}
	v.Estack().PushVal(header.Index)
	return nil
}

// headerGetHash returns header hash of the passed header.
func (ic *interopContext) headerGetHash(v *vm.VM) error {
	header, err := popHeaderFromVM(v)
	if err != nil {
		return err
	}
	v.Estack().PushVal(header.Hash().BytesBE())
	return nil
}

// headerGetPrevHash returns previous header hash of the passed header.
func (ic *interopContext) headerGetPrevHash(v *vm.VM) error {
	header, err := popHeaderFromVM(v)
	if err != nil {
		return err
	}
	v.Estack().PushVal(header.PrevHash.BytesBE())
	return nil
}

// headerGetTimestamp returns timestamp of the passed header.
func (ic *interopContext) headerGetTimestamp(v *vm.VM) error {
	header, err := popHeaderFromVM(v)
	if err != nil {
		return err
	}
	v.Estack().PushVal(header.Timestamp)
	return nil
}

// blockGetTransactionCount returns transactions count in the given block.
func (ic *interopContext) blockGetTransactionCount(v *vm.VM) error {
	blockInterface := v.Estack().Pop().Value()
	block, ok := blockInterface.(*block.Block)
	if !ok {
		return errors.New("value is not a block")
	}
	v.Estack().PushVal(len(block.Transactions))
	return nil
}

// blockGetTransactions returns transactions from the given block.
func (ic *interopContext) blockGetTransactions(v *vm.VM) error {
	blockInterface := v.Estack().Pop().Value()
	block, ok := blockInterface.(*block.Block)
	if !ok {
		return errors.New("value is not a block")
	}
	if len(block.Transactions) > vm.MaxArraySize {
		return errors.New("too many transactions")
	}
	txes := make([]vm.StackItem, 0, len(block.Transactions))
	for _, tx := range block.Transactions {
		txes = append(txes, vm.NewInteropItem(tx))
	}
	v.Estack().PushVal(txes)
	return nil
}

// blockGetTransaction returns transaction with the given number from the given
// block.
func (ic *interopContext) blockGetTransaction(v *vm.VM) error {
	blockInterface := v.Estack().Pop().Value()
	block, ok := blockInterface.(*block.Block)
	if !ok {
		return errors.New("value is not a block")
	}
	index := v.Estack().Pop().BigInt().Int64()
	if index < 0 || index >= int64(len(block.Transactions)) {
		return errors.New("wrong transaction index")
	}
	tx := block.Transactions[index]
	v.Estack().PushVal(vm.NewInteropItem(tx))
	return nil
}

// txGetHash returns transaction's hash.
func (ic *interopContext) txGetHash(v *vm.VM) error {
	txInterface := v.Estack().Pop().Value()
	tx, ok := txInterface.(*transaction.Transaction)
	if !ok {
		return errors.New("value is not a transaction")
	}
	v.Estack().PushVal(tx.Hash().BytesBE())
	return nil
}

// engineGetScriptContainer returns transaction that contains the script being
// run.
func (ic *interopContext) engineGetScriptContainer(v *vm.VM) error {
	v.Estack().PushVal(vm.NewInteropItem(ic.tx))
	return nil
}

// pushContextScriptHash returns script hash of the invocation stack element
// number n.
func getContextScriptHash(v *vm.VM, n int) util.Uint160 {
	ctxIface := v.Istack().Peek(n).Value()
	ctx := ctxIface.(*vm.Context)
	return ctx.ScriptHash()
}

// pushContextScriptHash pushes to evaluation stack the script hash of the
// invocation stack element number n.
func pushContextScriptHash(v *vm.VM, n int) error {
	h := getContextScriptHash(v, n)
	v.Estack().PushVal(h.BytesBE())
	return nil
}

// engineGetExecutingScriptHash returns executing script hash.
func (ic *interopContext) engineGetExecutingScriptHash(v *vm.VM) error {
	return pushContextScriptHash(v, 0)
}

// engineGetCallingScriptHash returns calling script hash.
func (ic *interopContext) engineGetCallingScriptHash(v *vm.VM) error {
	return pushContextScriptHash(v, 1)
}

// engineGetEntryScriptHash returns entry script hash.
func (ic *interopContext) engineGetEntryScriptHash(v *vm.VM) error {
	return pushContextScriptHash(v, v.Istack().Len()-1)
}

// runtimePlatform returns the name of the platform.
func (ic *interopContext) runtimePlatform(v *vm.VM) error {
	v.Estack().PushVal([]byte("NEO"))
	return nil
}

// runtimeGetTrigger returns the script trigger.
func (ic *interopContext) runtimeGetTrigger(v *vm.VM) error {
	v.Estack().PushVal(byte(ic.trigger))
	return nil
}

// checkHashedWitness checks given hash against current list of script hashes
// for verifying in the interop context.
func (ic *interopContext) checkHashedWitness(hash util.Uint160) (bool, error) {
	hashes, err := ic.bc.GetScriptHashesForVerifying(ic.tx)
	if err != nil {
		return false, gherr.Wrap(err, "failed to get script hashes")
	}
	for _, v := range hashes {
		if hash.Equals(v) {
			return true, nil
		}
	}
	return false, nil
}

// checkKeyedWitness checks hash of signature check contract with a given public
// key against current list of script hashes for verifying in the interop context.
func (ic *interopContext) checkKeyedWitness(key *keys.PublicKey) (bool, error) {
	return ic.checkHashedWitness(key.GetScriptHash())
}

// runtimeCheckWitness checks witnesses.
func (ic *interopContext) runtimeCheckWitness(v *vm.VM) error {
	var res bool
	var err error

	hashOrKey := v.Estack().Pop().Bytes()
	hash, err := util.Uint160DecodeBytesBE(hashOrKey)
	if err != nil {
		key := &keys.PublicKey{}
		err = key.DecodeBytes(hashOrKey)
		if err != nil {
			return errors.New("parameter given is neither a key nor a hash")
		}
		res, err = ic.checkKeyedWitness(key)
	} else {
		res, err = ic.checkHashedWitness(hash)
	}
	if err != nil {
		return gherr.Wrap(err, "failed to check")
	}
	v.Estack().PushVal(res)
	return nil
}

// runtimeNotify should pass stack item to the notify plugin to handle it, but
// in neo-go the only meaningful thing to do here is to log.
func (ic *interopContext) runtimeNotify(v *vm.VM) error {
	// It can be just about anything.
	e := v.Estack().Pop()
	item := e.Item()
	// But it has to be serializable, otherwise we either have some broken
	// (recursive) structure inside or an interop item that can't be used
	// outside of the interop subsystem anyway. I'd probably fail transactions
	// that emit such broken notifications, but that might break compatibility
	// with testnet/mainnet, so we're replacing these with error messages.
	_, err := vm.SerializeItem(item)
	if err != nil {
		item = vm.NewByteArrayItem([]byte(fmt.Sprintf("bad notification: %v", err)))
	}
	ne := state.NotificationEvent{ScriptHash: getContextScriptHash(v, 0), Item: item}
	ic.notifications = append(ic.notifications, ne)
	return nil
}

// runtimeLog logs the message passed.
func (ic *interopContext) runtimeLog(v *vm.VM) error {
	msg := fmt.Sprintf("%q", v.Estack().Pop().Bytes())
	ic.log.Info("runtime log",
		zap.Stringer("script", getContextScriptHash(v, 0)),
		zap.String("logs", msg))
	return nil
}

// runtimeGetTime returns timestamp of the block being verified, or the latest
// one in the blockchain if no block is given to interopContext.
func (ic *interopContext) runtimeGetTime(v *vm.VM) error {
	var header *block.Header
	if ic.block == nil {
		var err error
		header, err = ic.bc.GetHeader(ic.bc.CurrentBlockHash())
		if err != nil {
			return err
		}
	} else {
		header = ic.block.Header()
	}
	v.Estack().PushVal(header.Timestamp)
	return nil
}

/*
// runtimeSerialize serializes given stack item.
func (ic *interopContext) runtimeSerialize(v *vm.VM) error {
	panic("TODO")
}

// runtimeDeserialize deserializes given stack item.
func (ic *interopContext) runtimeDeserialize(v *vm.VM) error {
	panic("TODO")
}
*/
func (ic *interopContext) checkStorageContext(stc *StorageContext) error {
	contract, err := ic.dao.GetContractState(stc.ScriptHash)
	if err != nil {
		return errors.New("no contract found")
	}
	if !contract.HasStorage() {
		return fmt.Errorf("contract %s can't use storage", stc.ScriptHash)
	}
	return nil
}

// storageDelete deletes stored key-value pair.
func (ic *interopContext) storageDelete(v *vm.VM) error {
	if ic.trigger != trigger.Application && ic.trigger != trigger.ApplicationR {
		return errors.New("can't delete when the trigger is not application")
	}
	stcInterface := v.Estack().Pop().Value()
	stc, ok := stcInterface.(*StorageContext)
	if !ok {
		return fmt.Errorf("%T is not a StorageContext", stcInterface)
	}
	if stc.ReadOnly {
		return errors.New("StorageContext is read only")
	}
	err := ic.checkStorageContext(stc)
	if err != nil {
		return err
	}
	key := v.Estack().Pop().Bytes()
	si := ic.dao.GetStorageItem(stc.ScriptHash, key)
	if si != nil && si.IsConst {
		return errors.New("storage item is constant")
	}
	return ic.dao.DeleteStorageItem(stc.ScriptHash, key)
}

// storageGet returns stored key-value pair.
func (ic *interopContext) storageGet(v *vm.VM) error {
	stcInterface := v.Estack().Pop().Value()
	stc, ok := stcInterface.(*StorageContext)
	if !ok {
		return fmt.Errorf("%T is not a StorageContext", stcInterface)
	}
	err := ic.checkStorageContext(stc)
	if err != nil {
		return err
	}
	key := v.Estack().Pop().Bytes()
	si := ic.dao.GetStorageItem(stc.ScriptHash, key)
	if si != nil && si.Value != nil {
		v.Estack().PushVal(si.Value)
	} else {
		v.Estack().PushVal([]byte{})
	}
	return nil
}

// storageGetContext returns storage context (scripthash).
func (ic *interopContext) storageGetContext(v *vm.VM) error {
	sc := &StorageContext{
		ScriptHash: getContextScriptHash(v, 0),
		ReadOnly:   false,
	}
	v.Estack().PushVal(vm.NewInteropItem(sc))
	return nil
}

// storageGetReadOnlyContext returns read-only context (scripthash).
func (ic *interopContext) storageGetReadOnlyContext(v *vm.VM) error {
	sc := &StorageContext{
		ScriptHash: getContextScriptHash(v, 0),
		ReadOnly:   true,
	}
	v.Estack().PushVal(vm.NewInteropItem(sc))
	return nil
}

func (ic *interopContext) putWithContextAndFlags(stc *StorageContext, key []byte, value []byte, isConst bool) error {
	if ic.trigger != trigger.Application && ic.trigger != trigger.ApplicationR {
		return errors.New("can't delete when the trigger is not application")
	}
	if len(key) > MaxStorageKeyLen {
		return errors.New("key is too big")
	}
	if stc.ReadOnly {
		return errors.New("StorageContext is read only")
	}
	err := ic.checkStorageContext(stc)
	if err != nil {
		return err
	}
	si := ic.dao.GetStorageItem(stc.ScriptHash, key)
	if si == nil {
		si = &state.StorageItem{}
	}
	if si.IsConst {
		return errors.New("storage item exists and is read-only")
	}
	si.Value = value
	si.IsConst = isConst
	return ic.dao.PutStorageItem(stc.ScriptHash, key, si)
}

// storagePutInternal is a unified implementation of storagePut and storagePutEx.
func (ic *interopContext) storagePutInternal(v *vm.VM, getFlag bool) error {
	stcInterface := v.Estack().Pop().Value()
	stc, ok := stcInterface.(*StorageContext)
	if !ok {
		return fmt.Errorf("%T is not a StorageContext", stcInterface)
	}
	key := v.Estack().Pop().Bytes()
	value := v.Estack().Pop().Bytes()
	var flag int
	if getFlag {
		flag = int(v.Estack().Pop().BigInt().Int64())
	}
	return ic.putWithContextAndFlags(stc, key, value, flag == 1)
}

// storagePut puts key-value pair into the storage.
func (ic *interopContext) storagePut(v *vm.VM) error {
	return ic.storagePutInternal(v, false)
}

// storagePutEx puts key-value pair with given flags into the storage.
func (ic *interopContext) storagePutEx(v *vm.VM) error {
	return ic.storagePutInternal(v, true)
}

// storageContextAsReadOnly sets given context to read-only mode.
func (ic *interopContext) storageContextAsReadOnly(v *vm.VM) error {
	stcInterface := v.Estack().Pop().Value()
	stc, ok := stcInterface.(*StorageContext)
	if !ok {
		return fmt.Errorf("%T is not a StorageContext", stcInterface)
	}
	if !stc.ReadOnly {
		stx := &StorageContext{
			ScriptHash: stc.ScriptHash,
			ReadOnly:   true,
		}
		stc = stx
	}
	v.Estack().PushVal(vm.NewInteropItem(stc))
	return nil
}

// contractDestroy destroys a contract.
func (ic *interopContext) contractDestroy(v *vm.VM) error {
	if ic.trigger != trigger.Application {
		return errors.New("can't destroy contract when not triggered by application")
	}
	hash := getContextScriptHash(v, 0)
	cs, err := ic.dao.GetContractState(hash)
	if err != nil {
		return nil
	}
	err = ic.dao.DeleteContractState(hash)
	if err != nil {
		return err
	}
	if cs.HasStorage() {
		siMap, err := ic.dao.GetStorageItems(hash)
		if err != nil {
			return err
		}
		for k := range siMap {
			_ = ic.dao.DeleteStorageItem(hash, []byte(k))
		}
	}
	return nil
}

// contractGetStorageContext retrieves StorageContext of a contract.
func (ic *interopContext) contractGetStorageContext(v *vm.VM) error {
	csInterface := v.Estack().Pop().Value()
	cs, ok := csInterface.(*state.Contract)
	if !ok {
		return fmt.Errorf("%T is not a contract state", cs)
	}
	contractState, err := ic.dao.GetContractState(cs.ScriptHash())
	if contractState == nil || err != nil {
		return fmt.Errorf("contract was not created in this transaction")
	}
	stc := &StorageContext{
		ScriptHash: cs.ScriptHash(),
	}
	v.Estack().PushVal(vm.NewInteropItem(stc))
	return nil
}
