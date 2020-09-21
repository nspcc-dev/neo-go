package core

import (
	"crypto/elliptic"
	"errors"
	"fmt"
	"math"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/blockchainer"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"go.uber.org/zap"
)

const (
	// MaxStorageKeyLen is the maximum length of a key for storage items.
	MaxStorageKeyLen = 64
	// MaxStorageValueLen is the maximum length of a value for storage items.
	// It is set to be the maximum value for uint16.
	MaxStorageValueLen = 65535
	// MaxTraceableBlocks is the maximum number of blocks before current chain
	// height we're able to give information about.
	MaxTraceableBlocks = transaction.MaxValidUntilBlockIncrement
	// MaxEventNameLen is the maximum length of a name for event.
	MaxEventNameLen = 32
	// MaxNotificationSize is the maximum length of a runtime log message.
	MaxNotificationSize = 1024
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

// getBlockHashFromElement converts given vm.Element to block hash using given
// Blockchainer if needed. Interop functions accept both block numbers and
// block hashes as parameters, thus this function is needed.
func getBlockHashFromElement(bc blockchainer.Blockchainer, element *vm.Element) (util.Uint256, error) {
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

// blockToStackItem converts block.Block to stackitem.Item
func blockToStackItem(b *block.Block) stackitem.Item {
	return stackitem.NewArray([]stackitem.Item{
		stackitem.NewByteArray(b.Hash().BytesBE()),
		stackitem.NewBigInteger(big.NewInt(int64(b.Version))),
		stackitem.NewByteArray(b.PrevHash.BytesBE()),
		stackitem.NewByteArray(b.MerkleRoot.BytesBE()),
		stackitem.NewBigInteger(big.NewInt(int64(b.Timestamp))),
		stackitem.NewBigInteger(big.NewInt(int64(b.Index))),
		stackitem.NewByteArray(b.NextConsensus.BytesBE()),
		stackitem.NewBigInteger(big.NewInt(int64(len(b.Transactions)))),
	})
}

// bcGetBlock returns current block.
func bcGetBlock(ic *interop.Context) error {
	hash, err := getBlockHashFromElement(ic.Chain, ic.VM.Estack().Pop())
	if err != nil {
		return err
	}
	block, err := ic.Chain.GetBlock(hash)
	if err != nil || !isTraceableBlock(ic, block.Index) {
		ic.VM.Estack().PushVal(stackitem.Null{})
	} else {
		ic.VM.Estack().PushVal(blockToStackItem(block))
	}
	return nil
}

// contractToStackItem converts state.Contract to stackitem.Item
func contractToStackItem(cs *state.Contract) (stackitem.Item, error) {
	manifest, err := cs.Manifest.MarshalJSON()
	if err != nil {
		return nil, err
	}
	return stackitem.NewArray([]stackitem.Item{
		stackitem.NewByteArray(cs.Script),
		stackitem.NewByteArray(manifest),
		stackitem.NewBool(cs.HasStorage()),
		stackitem.NewBool(cs.IsPayable()),
	}), nil
}

// bcGetContract returns contract.
func bcGetContract(ic *interop.Context) error {
	hashbytes := ic.VM.Estack().Pop().Bytes()
	hash, err := util.Uint160DecodeBytesBE(hashbytes)
	if err != nil {
		return err
	}
	cs, err := ic.DAO.GetContractState(hash)
	if err != nil {
		ic.VM.Estack().PushVal(stackitem.Null{})
	} else {
		item, err := contractToStackItem(cs)
		if err != nil {
			return err
		}
		ic.VM.Estack().PushVal(item)
	}
	return nil
}

// bcGetHeight returns blockchain height.
func bcGetHeight(ic *interop.Context) error {
	ic.VM.Estack().PushVal(ic.Chain.BlockHeight())
	return nil
}

// getTransactionAndHeight gets parameter from the vm evaluation stack and
// returns transaction and its height if it's present in the blockchain.
func getTransactionAndHeight(cd *dao.Cached, v *vm.VM) (*transaction.Transaction, uint32, error) {
	hashbytes := v.Estack().Pop().Bytes()
	hash, err := util.Uint256DecodeBytesBE(hashbytes)
	if err != nil {
		return nil, 0, err
	}
	return cd.GetTransaction(hash)
}

// isTraceableBlock defines whether we're able to give information about
// the block with index specified.
func isTraceableBlock(ic *interop.Context, index uint32) bool {
	height := ic.Chain.BlockHeight()
	return index <= height && index+MaxTraceableBlocks > height
}

// transactionToStackItem converts transaction.Transaction to stackitem.Item
func transactionToStackItem(t *transaction.Transaction) stackitem.Item {
	return stackitem.NewArray([]stackitem.Item{
		stackitem.NewByteArray(t.Hash().BytesBE()),
		stackitem.NewBigInteger(big.NewInt(int64(t.Version))),
		stackitem.NewBigInteger(big.NewInt(int64(t.Nonce))),
		stackitem.NewByteArray(t.Sender().BytesBE()),
		stackitem.NewBigInteger(big.NewInt(int64(t.SystemFee))),
		stackitem.NewBigInteger(big.NewInt(int64(t.NetworkFee))),
		stackitem.NewBigInteger(big.NewInt(int64(t.ValidUntilBlock))),
		stackitem.NewByteArray(t.Script),
	})
}

// bcGetTransaction returns transaction.
func bcGetTransaction(ic *interop.Context) error {
	tx, h, err := getTransactionAndHeight(ic.DAO, ic.VM)
	if err != nil || !isTraceableBlock(ic, h) {
		ic.VM.Estack().PushVal(stackitem.Null{})
		return nil
	}
	ic.VM.Estack().PushVal(transactionToStackItem(tx))
	return nil
}

// bcGetTransactionFromBlock returns transaction with the given index from the
// block with height or hash specified.
func bcGetTransactionFromBlock(ic *interop.Context) error {
	hash, err := getBlockHashFromElement(ic.Chain, ic.VM.Estack().Pop())
	if err != nil {
		return err
	}
	index := ic.VM.Estack().Pop().BigInt().Int64()
	block, err := ic.DAO.GetBlock(hash)
	if err != nil || !isTraceableBlock(ic, block.Index) {
		ic.VM.Estack().PushVal(stackitem.Null{})
		return nil
	}
	if index < 0 || index >= int64(len(block.Transactions)) {
		return errors.New("wrong transaction index")
	}
	tx := block.Transactions[index]
	ic.VM.Estack().PushVal(tx.Hash().BytesBE())
	return nil
}

// bcGetTransactionHeight returns transaction height.
func bcGetTransactionHeight(ic *interop.Context) error {
	_, h, err := getTransactionAndHeight(ic.DAO, ic.VM)
	if err != nil || !isTraceableBlock(ic, h) {
		ic.VM.Estack().PushVal(-1)
		return nil
	}
	ic.VM.Estack().PushVal(h)
	return nil
}

// engineGetScriptContainer returns transaction or block that contains the script
// being run.
func engineGetScriptContainer(ic *interop.Context) error {
	var item stackitem.Item
	switch t := ic.Container.(type) {
	case *transaction.Transaction:
		item = transactionToStackItem(t)
	case *block.Block:
		item = blockToStackItem(t)
	default:
		return errors.New("unknown script container")
	}
	ic.VM.Estack().PushVal(item)
	return nil
}

// engineGetExecutingScriptHash returns executing script hash.
func engineGetExecutingScriptHash(ic *interop.Context) error {
	return ic.VM.PushContextScriptHash(0)
}

// engineGetCallingScriptHash returns calling script hash.
func engineGetCallingScriptHash(ic *interop.Context) error {
	return ic.VM.PushContextScriptHash(1)
}

// engineGetEntryScriptHash returns entry script hash.
func engineGetEntryScriptHash(ic *interop.Context) error {
	return ic.VM.PushContextScriptHash(ic.VM.Istack().Len() - 1)
}

// runtimePlatform returns the name of the platform.
func runtimePlatform(ic *interop.Context) error {
	ic.VM.Estack().PushVal([]byte("NEO"))
	return nil
}

// runtimeGetTrigger returns the script trigger.
func runtimeGetTrigger(ic *interop.Context) error {
	ic.VM.Estack().PushVal(byte(ic.Trigger))
	return nil
}

// runtimeNotify should pass stack item to the notify plugin to handle it, but
// in neo-go the only meaningful thing to do here is to log.
func runtimeNotify(ic *interop.Context) error {
	name := ic.VM.Estack().Pop().String()
	if len(name) > MaxEventNameLen {
		return fmt.Errorf("event name must be less than %d", MaxEventNameLen)
	}
	elem := ic.VM.Estack().Pop()
	args := elem.Array()
	// But it has to be serializable, otherwise we either have some broken
	// (recursive) structure inside or an interop item that can't be used
	// outside of the interop subsystem anyway. I'd probably fail transactions
	// that emit such broken notifications, but that might break compatibility
	// with testnet/mainnet, so we're replacing these with error messages.
	_, err := stackitem.SerializeItem(elem.Item())
	if err != nil {
		args = []stackitem.Item{stackitem.NewByteArray([]byte(fmt.Sprintf("bad notification: %v", err)))}
	}
	ne := state.NotificationEvent{
		ScriptHash: ic.VM.GetCurrentScriptHash(),
		Name:       name,
		Item:       stackitem.DeepCopy(stackitem.NewArray(args)).(*stackitem.Array),
	}
	ic.Notifications = append(ic.Notifications, ne)
	return nil
}

// runtimeLog logs the message passed.
func runtimeLog(ic *interop.Context) error {
	state := ic.VM.Estack().Pop().String()
	if len(state) > MaxNotificationSize {
		return fmt.Errorf("message length shouldn't exceed %v", MaxNotificationSize)
	}
	msg := fmt.Sprintf("%q", state)
	ic.Log.Info("runtime log",
		zap.Stringer("script", ic.VM.GetCurrentScriptHash()),
		zap.String("logs", msg))
	return nil
}

// runtimeGetTime returns timestamp of the block being verified, or the latest
// one in the blockchain if no block is given to Context.
func runtimeGetTime(ic *interop.Context) error {
	var header *block.Header
	if ic.Block == nil {
		var err error
		header, err = ic.Chain.GetHeader(ic.Chain.CurrentBlockHash())
		if err != nil {
			return err
		}
	} else {
		header = ic.Block.Header()
	}
	ic.VM.Estack().PushVal(header.Timestamp)
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
	contract, err := ic.DAO.GetContractState(ic.VM.GetCurrentScriptHash())
	if err != nil {
		return err
	}
	if !contract.HasStorage() {
		return errors.New("contract is not allowed to use storage")
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
	if si == nil {
		si = &state.StorageItem{}
	}
	if si.IsConst {
		return errors.New("storage item exists and is read-only")
	}
	sizeInc := 1
	if len(value) > len(si.Value) {
		sizeInc = len(value) - len(si.Value)
	}
	if !ic.VM.AddGas(int64(sizeInc) * StoragePrice) {
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

// contractDestroy destroys a contract.
func contractDestroy(ic *interop.Context) error {
	hash := ic.VM.GetCurrentScriptHash()
	cs, err := ic.DAO.GetContractState(hash)
	if err != nil {
		return nil
	}
	err = ic.DAO.DeleteContractState(hash)
	if err != nil {
		return err
	}
	if cs.HasStorage() {
		siMap, err := ic.DAO.GetStorageItems(cs.ID)
		if err != nil {
			return err
		}
		for k := range siMap {
			_ = ic.DAO.DeleteStorageItem(cs.ID, []byte(k))
		}
	}
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
	cs, _ := ic.DAO.GetContractState(u)
	if cs != nil {
		result = vm.IsStandardContract(cs.Script)
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
