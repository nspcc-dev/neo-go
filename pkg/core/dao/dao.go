package dao

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"

	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/mpt"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// DAO is a data access object.
type DAO interface {
	AppendNEP5Transfer(acc util.Uint160, index uint32, tr *state.NEP5Transfer) (bool, error)
	AppendTransfer(acc util.Uint160, index uint32, tr *state.Transfer) (bool, error)
	DeleteContractState(hash util.Uint160) error
	DeleteStorageItem(scripthash util.Uint160, key []byte) error
	DeleteValidatorState(vs *state.Validator) error
	GetAccountState(hash util.Uint160) (*state.Account, error)
	GetAccountStateOrNew(hash util.Uint160) (*state.Account, error)
	GetAndDecode(entity io.Serializable, key []byte) error
	GetAppExecResult(hash util.Uint256) (*state.AppExecResult, error)
	GetAssetState(assetID util.Uint256) (*state.Asset, error)
	GetBatch() *storage.MemBatch
	GetBlock(hash util.Uint256) (*block.Block, uint32, error)
	GetContractState(hash util.Uint160) (*state.Contract, error)
	GetCurrentBlockHeight() (uint32, error)
	GetCurrentHeaderHeight() (i uint32, h util.Uint256, err error)
	GetCurrentStateRootHeight() (uint32, error)
	GetHeaderHashes() ([]util.Uint256, error)
	GetNEP5Balances(acc util.Uint160) (*state.NEP5Balances, error)
	GetNEP5Metadata(h util.Uint160) (*state.NEP5Metadata, error)
	GetNEP5TransferLog(acc util.Uint160, index uint32) (*state.TransferLog, error)
	GetNextTransferBatch(acc util.Uint160) (uint32, error)
	GetStateRoot(height uint32) (*state.MPTRootState, error)
	PutStateRoot(root *state.MPTRootState) error
	GetStorageItem(scripthash util.Uint160, key []byte) *state.StorageItem
	GetStorageItems(hash util.Uint160, prefix []byte) ([]StorageItemWithKey, error)
	GetTransaction(hash util.Uint256) (*transaction.Transaction, uint32, error)
	GetTransferLog(acc util.Uint160, index uint32) (*state.TransferLog, error)
	GetUnspentCoinState(hash util.Uint256) (*state.UnspentCoin, error)
	GetValidatorState(publicKey *keys.PublicKey) (*state.Validator, error)
	GetValidatorStateOrNew(publicKey *keys.PublicKey) (*state.Validator, error)
	GetValidators() []*state.Validator
	GetValidatorsCount() (*state.ValidatorsCount, error)
	GetVersion() (string, error)
	GetWrapped() DAO
	HasTransaction(hash util.Uint256) bool
	IsDoubleClaim(claim *transaction.ClaimTX) bool
	IsDoubleSpend(tx *transaction.Transaction) bool
	Persist() (int, error)
	PutAccountState(as *state.Account) error
	PutAppExecResult(aer *state.AppExecResult) error
	PutAssetState(as *state.Asset) error
	PutContractState(cs *state.Contract) error
	PutCurrentHeader(hashAndIndex []byte) error
	PutNEP5Balances(acc util.Uint160, bs *state.NEP5Balances) error
	PutNEP5Metadata(h util.Uint160, meta *state.NEP5Metadata) error
	PutNEP5TransferLog(acc util.Uint160, index uint32, lg *state.TransferLog) error
	PutNextTransferBatch(acc util.Uint160, num uint32) error
	PutStorageItem(scripthash util.Uint160, key []byte, si *state.StorageItem) error
	PutTransferLog(acc util.Uint160, index uint32, lg *state.TransferLog) error
	PutUnspentCoinState(hash util.Uint256, ucs *state.UnspentCoin) error
	PutValidatorState(vs *state.Validator) error
	PutValidatorsCount(vc *state.ValidatorsCount) error
	PutVersion(v string) error
	StoreAsBlock(block *block.Block, sysFee uint32) error
	StoreAsCurrentBlock(block *block.Block) error
	StoreAsTransaction(tx *transaction.Transaction, index uint32) error
	putAccountState(as *state.Account, buf *io.BufBinWriter) error
	putNEP5Balances(acc util.Uint160, bs *state.NEP5Balances, buf *io.BufBinWriter) error
	putUnspentCoinState(hash util.Uint256, ucs *state.UnspentCoin, buf *io.BufBinWriter) error
}

// Simple is memCached wrapper around DB, simple DAO implementation.
type Simple struct {
	MPT   *mpt.Trie
	Store *storage.MemCachedStore
}

// NewSimple creates new simple dao using provided backend store.
func NewSimple(backend storage.Store) *Simple {
	st := storage.NewMemCachedStore(backend)
	return &Simple{Store: st}
}

// GetBatch returns currently accumulated DB changeset.
func (dao *Simple) GetBatch() *storage.MemBatch {
	return dao.Store.GetBatch()
}

// GetWrapped returns new DAO instance with another layer of wrapped
// MemCachedStore around the current DAO Store.
func (dao *Simple) GetWrapped() DAO {
	d := NewSimple(dao.Store)
	d.MPT = dao.MPT
	return d
}

// GetAndDecode performs get operation and decoding with serializable structures.
func (dao *Simple) GetAndDecode(entity io.Serializable, key []byte) error {
	entityBytes, err := dao.Store.Get(key)
	if err != nil {
		return err
	}
	reader := io.NewBinReaderFromBuf(entityBytes)
	entity.DecodeBinary(reader)
	return reader.Err
}

// Put performs put operation with serializable structures.
func (dao *Simple) Put(entity io.Serializable, key []byte) error {
	return dao.putWithBuffer(entity, key, io.NewBufBinWriter())
}

// putWithBuffer performs put operation using buf as a pre-allocated buffer for serialization.
func (dao *Simple) putWithBuffer(entity io.Serializable, key []byte, buf *io.BufBinWriter) error {
	entity.EncodeBinary(buf.BinWriter)
	if buf.Err != nil {
		return buf.Err
	}
	return dao.Store.Put(key, buf.Bytes())
}

// -- start accounts.

// GetAccountStateOrNew retrieves Account from temporary or persistent Store
// or creates a new one if it doesn't exist and persists it.
func (dao *Simple) GetAccountStateOrNew(hash util.Uint160) (*state.Account, error) {
	account, err := dao.GetAccountState(hash)
	if err != nil {
		if err != storage.ErrKeyNotFound {
			return nil, err
		}
		account = state.NewAccount(hash)
	}
	return account, nil
}

// GetAccountState returns Account from the given Store if it's
// present there. Returns nil otherwise.
func (dao *Simple) GetAccountState(hash util.Uint160) (*state.Account, error) {
	account := &state.Account{}
	key := storage.AppendPrefix(storage.STAccount, hash.BytesBE())
	err := dao.GetAndDecode(account, key)
	if err != nil {
		return nil, err
	}
	return account, err
}

// PutAccountState saves given Account in given store.
func (dao *Simple) PutAccountState(as *state.Account) error {
	return dao.putAccountState(as, io.NewBufBinWriter())
}

func (dao *Simple) putAccountState(as *state.Account, buf *io.BufBinWriter) error {
	key := storage.AppendPrefix(storage.STAccount, as.ScriptHash.BytesBE())
	return dao.putWithBuffer(as, key, buf)
}

// -- end accounts.

// -- start assets.

// GetAssetState returns given asset state as recorded in the given store.
func (dao *Simple) GetAssetState(assetID util.Uint256) (*state.Asset, error) {
	asset := &state.Asset{}
	key := storage.AppendPrefix(storage.STAsset, assetID.BytesBE())
	err := dao.GetAndDecode(asset, key)
	if err != nil {
		return nil, err
	}
	if asset.ID != assetID {
		return nil, fmt.Errorf("found asset id is not equal to expected")
	}
	return asset, nil
}

// PutAssetState puts given asset state into the given store.
func (dao *Simple) PutAssetState(as *state.Asset) error {
	key := storage.AppendPrefix(storage.STAsset, as.ID.BytesBE())
	return dao.Put(as, key)
}

// -- end assets.

// -- start contracts.

// GetContractState returns contract state as recorded in the given
// store by the given script hash.
func (dao *Simple) GetContractState(hash util.Uint160) (*state.Contract, error) {
	contract := &state.Contract{}
	key := storage.AppendPrefix(storage.STContract, hash.BytesBE())
	err := dao.GetAndDecode(contract, key)
	if err != nil {
		return nil, err
	}
	if contract.ScriptHash() != hash {
		return nil, fmt.Errorf("found script hash is not equal to expected")
	}

	return contract, nil
}

// PutContractState puts given contract state into the given store.
func (dao *Simple) PutContractState(cs *state.Contract) error {
	key := storage.AppendPrefix(storage.STContract, cs.ScriptHash().BytesBE())
	return dao.Put(cs, key)
}

// DeleteContractState deletes given contract state in the given store.
func (dao *Simple) DeleteContractState(hash util.Uint160) error {
	key := storage.AppendPrefix(storage.STContract, hash.BytesBE())
	return dao.Store.Delete(key)
}

// GetNEP5Metadata returns saved NEP5 metadata for the contract h.
func (dao *Simple) GetNEP5Metadata(h util.Uint160) (*state.NEP5Metadata, error) {
	key := storage.AppendPrefix(storage.STMigration, h.BytesBE())
	m := new(state.NEP5Metadata)
	if err := dao.GetAndDecode(m, key); err != nil {
		return nil, err
	}
	return m, nil
}

// PutNEP5Metadata saves NEP5 metadata for the contract h.
func (dao *Simple) PutNEP5Metadata(h util.Uint160, m *state.NEP5Metadata) error {
	key := storage.AppendPrefix(storage.STMigration, h.BytesBE())
	return dao.Put(m, key)
}

// -- end contracts.

// -- start nep5 balances.

// GetNEP5Balances retrieves nep5 balances from the cache.
func (dao *Simple) GetNEP5Balances(acc util.Uint160) (*state.NEP5Balances, error) {
	key := storage.AppendPrefix(storage.STNEP5Balances, acc.BytesBE())
	bs := state.NewNEP5Balances()
	err := dao.GetAndDecode(bs, key)
	if err != nil && err != storage.ErrKeyNotFound {
		return nil, err
	}
	return bs, nil
}

// PutNEP5Balances saves nep5 balances from the cache.
func (dao *Simple) PutNEP5Balances(acc util.Uint160, bs *state.NEP5Balances) error {
	return dao.putNEP5Balances(acc, bs, io.NewBufBinWriter())
}

func (dao *Simple) putNEP5Balances(acc util.Uint160, bs *state.NEP5Balances, buf *io.BufBinWriter) error {
	key := storage.AppendPrefix(storage.STNEP5Balances, acc.BytesBE())
	return dao.putWithBuffer(bs, key, buf)
}

// -- end nep5 balances.

// -- start transfer log.

const nep5TransferBatchSize = 128 * state.NEP5TransferSize
const transferBatchSize = 128 * state.TransferSize

func getTransferLogKey(acc util.Uint160, index uint32) []byte {
	key := make([]byte, 1+util.Uint160Size+4)
	key[0] = byte(storage.STTransfers)
	copy(key[1:], acc.BytesBE())
	binary.LittleEndian.PutUint32(key[util.Uint160Size:], index)
	return key
}

func getNEP5TransferLogKey(acc util.Uint160, index uint32) []byte {
	key := make([]byte, 1+util.Uint160Size+4)
	key[0] = byte(storage.STNEP5Transfers)
	copy(key[1:], acc.BytesBE())
	binary.LittleEndian.PutUint32(key[util.Uint160Size:], index)
	return key
}

// GetNextTransferBatch returns index for the transfer batch to write to.
func (dao *Simple) GetNextTransferBatch(acc util.Uint160) (uint32, error) {
	key := storage.AppendPrefix(storage.STTransfers, acc.BytesBE())
	val, err := dao.Store.Get(key)
	if err != nil {
		if err != storage.ErrKeyNotFound {
			return 0, err
		}
		return 0, nil
	}
	return binary.LittleEndian.Uint32(val), nil
}

// PutNextTransferBatch sets index of the transfer batch to write to.
func (dao *Simple) PutNextTransferBatch(acc util.Uint160, num uint32) error {
	key := storage.AppendPrefix(storage.STTransfers, acc.BytesBE())
	val := make([]byte, 4)
	binary.LittleEndian.PutUint32(val, num)
	return dao.Store.Put(key, val)
}

// GetTransferLog retrieves transfer log from the cache.
func (dao *Simple) GetTransferLog(acc util.Uint160, index uint32) (*state.TransferLog, error) {
	key := getTransferLogKey(acc, index)
	value, err := dao.Store.Get(key)
	if err != nil {
		if err == storage.ErrKeyNotFound {
			return new(state.TransferLog), nil
		}
		return nil, err
	}
	return &state.TransferLog{Raw: value}, nil
}

// PutTransferLog saves given transfer log in the cache.
func (dao *Simple) PutTransferLog(acc util.Uint160, index uint32, lg *state.TransferLog) error {
	key := getTransferLogKey(acc, index)
	return dao.Store.Put(key, lg.Raw)
}

// AppendTransfer appends a single transfer to a log.
// First return value signalizes that log size has exceeded batch size.
func (dao *Simple) AppendTransfer(acc util.Uint160, index uint32, tr *state.Transfer) (bool, error) {
	lg, err := dao.GetTransferLog(acc, index)
	if err != nil {
		if err != storage.ErrKeyNotFound {
			return false, err
		}
		lg = new(state.TransferLog)
	}
	if err := lg.Append(tr); err != nil {
		return false, err
	}
	return lg.Size() >= transferBatchSize, dao.PutTransferLog(acc, index, lg)
}

// GetNEP5TransferLog retrieves transfer log from the cache.
func (dao *Simple) GetNEP5TransferLog(acc util.Uint160, index uint32) (*state.TransferLog, error) {
	key := getNEP5TransferLogKey(acc, index)
	value, err := dao.Store.Get(key)
	if err != nil {
		if err == storage.ErrKeyNotFound {
			return new(state.TransferLog), nil
		}
		return nil, err
	}
	return &state.TransferLog{Raw: value}, nil
}

// PutNEP5TransferLog saves given transfer log in the cache.
func (dao *Simple) PutNEP5TransferLog(acc util.Uint160, index uint32, lg *state.TransferLog) error {
	key := getNEP5TransferLogKey(acc, index)
	return dao.Store.Put(key, lg.Raw)
}

// AppendNEP5Transfer appends a single NEP5 transfer to a log.
// First return value signalizes that log size has exceeded batch size.
func (dao *Simple) AppendNEP5Transfer(acc util.Uint160, index uint32, tr *state.NEP5Transfer) (bool, error) {
	lg, err := dao.GetNEP5TransferLog(acc, index)
	if err != nil {
		if err != storage.ErrKeyNotFound {
			return false, err
		}
		lg = new(state.TransferLog)
	}
	if err := lg.Append(tr); err != nil {
		return false, err
	}
	return lg.Size() >= nep5TransferBatchSize, dao.PutNEP5TransferLog(acc, index, lg)
}

// -- end transfer log.

// -- start unspent coins.

// GetUnspentCoinState retrieves UnspentCoinState from the given store.
func (dao *Simple) GetUnspentCoinState(hash util.Uint256) (*state.UnspentCoin, error) {
	unspent := &state.UnspentCoin{}
	key := storage.AppendPrefix(storage.STCoin, hash.BytesLE())
	err := dao.GetAndDecode(unspent, key)
	if err != nil {
		return nil, err
	}
	return unspent, nil
}

// PutUnspentCoinState puts given UnspentCoinState into the given store.
func (dao *Simple) PutUnspentCoinState(hash util.Uint256, ucs *state.UnspentCoin) error {
	return dao.putUnspentCoinState(hash, ucs, io.NewBufBinWriter())
}

func (dao *Simple) putUnspentCoinState(hash util.Uint256, ucs *state.UnspentCoin, buf *io.BufBinWriter) error {
	key := storage.AppendPrefix(storage.STCoin, hash.BytesLE())
	return dao.putWithBuffer(ucs, key, buf)
}

// -- end unspent coins.

// -- start validator.

// GetValidatorStateOrNew gets validator from store or created new one in case of error.
func (dao *Simple) GetValidatorStateOrNew(publicKey *keys.PublicKey) (*state.Validator, error) {
	validatorState, err := dao.GetValidatorState(publicKey)
	if err != nil {
		if err != storage.ErrKeyNotFound {
			return nil, err
		}
		validatorState = &state.Validator{PublicKey: publicKey}
	}
	return validatorState, nil

}

// GetValidators returns all validators from store.
func (dao *Simple) GetValidators() []*state.Validator {
	var validators []*state.Validator
	dao.Store.Seek(storage.STValidator.Bytes(), func(k, v []byte) {
		r := io.NewBinReaderFromBuf(v)
		validator := &state.Validator{}
		validator.DecodeBinary(r)
		if r.Err != nil {
			return
		}
		validators = append(validators, validator)
	})
	return validators
}

// GetValidatorState returns validator by publicKey.
func (dao *Simple) GetValidatorState(publicKey *keys.PublicKey) (*state.Validator, error) {
	validatorState := &state.Validator{}
	key := storage.AppendPrefix(storage.STValidator, publicKey.Bytes())
	err := dao.GetAndDecode(validatorState, key)
	if err != nil {
		return nil, err
	}
	return validatorState, nil
}

// PutValidatorState puts given Validator into the given store.
func (dao *Simple) PutValidatorState(vs *state.Validator) error {
	key := storage.AppendPrefix(storage.STValidator, vs.PublicKey.Bytes())
	return dao.Put(vs, key)
}

// DeleteValidatorState deletes given Validator into the given store.
func (dao *Simple) DeleteValidatorState(vs *state.Validator) error {
	key := storage.AppendPrefix(storage.STValidator, vs.PublicKey.Bytes())
	return dao.Store.Delete(key)
}

// GetValidatorsCount returns current ValidatorsCount or new one if there is none
// in the DB.
func (dao *Simple) GetValidatorsCount() (*state.ValidatorsCount, error) {
	vc := &state.ValidatorsCount{}
	key := []byte{byte(storage.IXValidatorsCount)}
	err := dao.GetAndDecode(vc, key)
	if err != nil && err != storage.ErrKeyNotFound {
		return nil, err
	}
	return vc, nil
}

// PutValidatorsCount put given ValidatorsCount in the store.
func (dao *Simple) PutValidatorsCount(vc *state.ValidatorsCount) error {
	key := []byte{byte(storage.IXValidatorsCount)}
	return dao.Put(vc, key)
}

// -- end validator.

// -- start notification event.

// GetAppExecResult gets application execution result from the
// given store.
func (dao *Simple) GetAppExecResult(hash util.Uint256) (*state.AppExecResult, error) {
	aer := &state.AppExecResult{}
	key := storage.AppendPrefix(storage.STNotification, hash.BytesBE())
	err := dao.GetAndDecode(aer, key)
	if err != nil {
		return nil, err
	}
	return aer, nil
}

// PutAppExecResult puts given application execution result into the
// given store.
func (dao *Simple) PutAppExecResult(aer *state.AppExecResult) error {
	key := storage.AppendPrefix(storage.STNotification, aer.TxHash.BytesBE())
	return dao.Put(aer, key)
}

// -- end notification event.

// -- start storage item.

func makeStateRootKey(height uint32) []byte {
	key := make([]byte, 5)
	key[0] = byte(storage.DataMPT)
	binary.LittleEndian.PutUint32(key[1:], height)
	return key
}

// InitMPT initializes MPT at the given height.
func (dao *Simple) InitMPT(height uint32, enableRefCount bool) error {
	var gcKey = []byte{byte(storage.DataMPT), 1}
	if height == 0 {
		dao.MPT = mpt.NewTrie(nil, enableRefCount, dao.Store)
		var val byte
		if enableRefCount {
			val = 1
		}
		return dao.Store.Put(gcKey, []byte{val})
	}
	var hasRefCount bool
	if v, err := dao.Store.Get(gcKey); err == nil {
		hasRefCount = v[0] != 0
	}
	if hasRefCount != enableRefCount {
		return fmt.Errorf("KeepOnlyLatestState setting mismatch: old=%v, new=%v", hasRefCount, enableRefCount)
	}
	r, err := dao.GetStateRoot(height)
	if err != nil {
		return err
	}
	var rootnode mpt.Node
	if !r.Root.Equals(util.Uint256{}) { // some initial blocks can have root == 0 and it's not a valid root
		rootnode = mpt.NewHashNode(r.Root)
	}
	dao.MPT = mpt.NewTrie(rootnode, enableRefCount, dao.Store)
	return nil
}

// GetCurrentStateRootHeight returns current state root height.
func (dao *Simple) GetCurrentStateRootHeight() (uint32, error) {
	key := []byte{byte(storage.DataMPT)}
	val, err := dao.Store.Get(key)
	if err != nil {
		if err == storage.ErrKeyNotFound {
			err = nil
		}
		return 0, err
	}
	return binary.LittleEndian.Uint32(val), nil
}

// PutCurrentStateRootHeight updates current state root height.
func (dao *Simple) PutCurrentStateRootHeight(height uint32) error {
	key := []byte{byte(storage.DataMPT)}
	val := make([]byte, 4)
	binary.LittleEndian.PutUint32(val, height)
	return dao.Store.Put(key, val)
}

// GetStateRoot returns state root of a given height.
func (dao *Simple) GetStateRoot(height uint32) (*state.MPTRootState, error) {
	r := new(state.MPTRootState)
	err := dao.GetAndDecode(r, makeStateRootKey(height))
	if err != nil {
		return nil, err
	}
	return r, nil
}

// PutStateRoot puts state root of a given height into the store.
func (dao *Simple) PutStateRoot(r *state.MPTRootState) error {
	return dao.Put(r, makeStateRootKey(r.Index))
}

// GetStorageItem returns StorageItem if it exists in the given store.
func (dao *Simple) GetStorageItem(scripthash util.Uint160, key []byte) *state.StorageItem {
	b, err := dao.Store.Get(makeStorageItemKey(scripthash, key))
	if err != nil {
		return nil
	}
	r := io.NewBinReaderFromBuf(b)

	si := &state.StorageItem{}
	si.DecodeBinary(r)
	if r.Err != nil {
		return nil
	}

	return si
}

// PutStorageItem puts given StorageItem for given script with given
// key into the given store.
func (dao *Simple) PutStorageItem(scripthash util.Uint160, key []byte, si *state.StorageItem) error {
	stKey := makeStorageItemKey(scripthash, key)
	v := mpt.ToNeoStorageValue(si)
	if dao.MPT != nil {
		k := mpt.ToNeoStorageKey(stKey[1:]) // strip STStorage prefix
		if err := dao.MPT.Put(k, v); err != nil && err != mpt.ErrNotFound {
			return err
		}
	}
	return dao.Store.Put(stKey, v[1:])
}

// DeleteStorageItem drops storage item for the given script with the
// given key from the store.
func (dao *Simple) DeleteStorageItem(scripthash util.Uint160, key []byte) error {
	stKey := makeStorageItemKey(scripthash, key)
	if dao.MPT != nil {
		k := mpt.ToNeoStorageKey(stKey[1:]) // strip STStorage prefix
		if err := dao.MPT.Delete(k); err != nil && err != mpt.ErrNotFound {
			return err
		}
	}
	return dao.Store.Delete(stKey)
}

// StorageItemWithKey is a Key-Value pair together with possible const modifier.
type StorageItemWithKey struct {
	state.StorageItem
	Key []byte
}

// GetStorageItems returns all storage items for a given scripthash.
func (dao *Simple) GetStorageItems(hash util.Uint160, prefix []byte) ([]StorageItemWithKey, error) {
	var res []StorageItemWithKey
	var err error

	saveToMap := func(k, v []byte) {
		if err != nil {
			return
		}
		r := io.NewBinReaderFromBuf(v)
		var s StorageItemWithKey
		s.StorageItem.DecodeBinary(r)
		if r.Err != nil {
			err = r.Err
			return
		}

		// Cut prefix and hash.
		// Must copy here, #1468.
		s.Key = make([]byte, len(k[21:]))
		copy(s.Key, k[21:])
		res = append(res, s)
	}
	dao.Store.Seek(makeStorageItemKey(hash, prefix), saveToMap)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// makeStorageItemKey returns a key used to store StorageItem in the DB.
func makeStorageItemKey(scripthash util.Uint160, key []byte) []byte {
	return storage.AppendPrefix(storage.STStorage, append(scripthash.BytesLE(), key...))
}

// -- end storage item.

// -- other.

// GetBlock returns Block by the given hash if it exists in the store.
func (dao *Simple) GetBlock(hash util.Uint256) (*block.Block, uint32, error) {
	key := storage.AppendPrefix(storage.DataBlock, hash.BytesLE())
	b, err := dao.Store.Get(key)
	if err != nil {
		return nil, 0, err
	}

	block, err := block.NewBlockFromTrimmedBytes(b[4:])
	if err != nil {
		return nil, 0, err
	}
	return block, binary.LittleEndian.Uint32(b[:4]), nil
}

// GetVersion attempts to get the current version stored in the
// underlying store.
func (dao *Simple) GetVersion() (string, error) {
	version, err := dao.Store.Get(storage.SYSVersion.Bytes())
	return string(version), err
}

// GetCurrentBlockHeight returns the current block height found in the
// underlying store.
func (dao *Simple) GetCurrentBlockHeight() (uint32, error) {
	b, err := dao.Store.Get(storage.SYSCurrentBlock.Bytes())
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(b[32:36]), nil
}

// GetCurrentHeaderHeight returns the current header height and hash from
// the underlying store.
func (dao *Simple) GetCurrentHeaderHeight() (i uint32, h util.Uint256, err error) {
	var b []byte
	b, err = dao.Store.Get(storage.SYSCurrentHeader.Bytes())
	if err != nil {
		return
	}
	i = binary.LittleEndian.Uint32(b[32:36])
	h, err = util.Uint256DecodeBytesLE(b[:32])
	return
}

// GetHeaderHashes returns a sorted list of header hashes retrieved from
// the given underlying store.
func (dao *Simple) GetHeaderHashes() ([]util.Uint256, error) {
	hashMap := make(map[uint32][]util.Uint256)
	dao.Store.Seek(storage.IXHeaderHashList.Bytes(), func(k, v []byte) {
		storedCount := binary.LittleEndian.Uint32(k[1:])
		hashes, err := read2000Uint256Hashes(v)
		if err != nil {
			panic(err)
		}
		hashMap[storedCount] = hashes
	})

	var (
		hashes     = make([]util.Uint256, 0, len(hashMap))
		sortedKeys = make([]uint32, 0, len(hashMap))
	)

	for k := range hashMap {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Slice(sortedKeys, func(i, j int) bool { return sortedKeys[i] < sortedKeys[j] })

	for _, key := range sortedKeys {
		hashes = append(hashes[:key], hashMap[key]...)
	}

	return hashes, nil
}

// GetTransaction returns Transaction and its height by the given hash
// if it exists in the store.
func (dao *Simple) GetTransaction(hash util.Uint256) (*transaction.Transaction, uint32, error) {
	key := storage.AppendPrefix(storage.DataTransaction, hash.BytesLE())
	b, err := dao.Store.Get(key)
	if err != nil {
		return nil, 0, err
	}
	r := io.NewBinReaderFromBuf(b)

	var height = r.ReadU32LE()

	tx := &transaction.Transaction{}
	tx.DecodeBinary(r)
	if r.Err != nil {
		return nil, 0, r.Err
	}

	return tx, height, nil
}

// PutVersion stores the given version in the underlying store.
func (dao *Simple) PutVersion(v string) error {
	return dao.Store.Put(storage.SYSVersion.Bytes(), []byte(v))
}

// PutCurrentHeader stores current header.
func (dao *Simple) PutCurrentHeader(hashAndIndex []byte) error {
	return dao.Store.Put(storage.SYSCurrentHeader.Bytes(), hashAndIndex)
}

// read2000Uint256Hashes attempts to read 2000 Uint256 hashes from
// the given byte array.
func read2000Uint256Hashes(b []byte) ([]util.Uint256, error) {
	r := bytes.NewReader(b)
	br := io.NewBinReaderFromIO(r)
	hashes := make([]util.Uint256, 0)
	br.ReadArray(&hashes)
	if br.Err != nil {
		return nil, br.Err
	}
	return hashes, nil
}

// HasTransaction returns true if the given store contains the given
// Transaction hash.
func (dao *Simple) HasTransaction(hash util.Uint256) bool {
	key := storage.AppendPrefix(storage.DataTransaction, hash.BytesLE())
	if _, err := dao.Store.Get(key); err == nil {
		return true
	}
	return false
}

// StoreAsBlock stores the given block as DataBlock.
func (dao *Simple) StoreAsBlock(block *block.Block, sysFee uint32) error {
	var (
		key = storage.AppendPrefix(storage.DataBlock, block.Hash().BytesLE())
		buf = io.NewBufBinWriter()
	)
	buf.WriteU32LE(sysFee)
	b, err := block.Trim()
	if err != nil {
		return err
	}
	buf.WriteBytes(b)
	if buf.Err != nil {
		return buf.Err
	}
	return dao.Store.Put(key, buf.Bytes())
}

// StoreAsCurrentBlock stores the given block witch prefix SYSCurrentBlock.
func (dao *Simple) StoreAsCurrentBlock(block *block.Block) error {
	buf := io.NewBufBinWriter()
	h := block.Hash()
	h.EncodeBinary(buf.BinWriter)
	buf.WriteU32LE(block.Index)
	return dao.Store.Put(storage.SYSCurrentBlock.Bytes(), buf.Bytes())
}

// StoreAsTransaction stores the given TX as DataTransaction.
func (dao *Simple) StoreAsTransaction(tx *transaction.Transaction, index uint32) error {
	key := storage.AppendPrefix(storage.DataTransaction, tx.Hash().BytesLE())
	buf := io.NewBufBinWriter()
	buf.WriteU32LE(index)
	tx.EncodeBinary(buf.BinWriter)
	if buf.Err != nil {
		return buf.Err
	}
	return dao.Store.Put(key, buf.Bytes())
}

// IsDoubleSpend verifies that the input transactions are not double spent.
func (dao *Simple) IsDoubleSpend(tx *transaction.Transaction) bool {
	return dao.checkUsedInputs(tx.Inputs, state.CoinSpent)
}

// IsDoubleClaim verifies that given claim inputs are not already claimed by another tx.
func (dao *Simple) IsDoubleClaim(claim *transaction.ClaimTX) bool {
	return dao.checkUsedInputs(claim.Claims, state.CoinClaimed)
}

func (dao *Simple) checkUsedInputs(inputs []transaction.Input, coin state.Coin) bool {
	if len(inputs) == 0 {
		return false
	}
	for _, inputs := range transaction.GroupInputsByPrevHash(inputs) {
		prevHash := inputs[0].PrevHash
		unspent, err := dao.GetUnspentCoinState(prevHash)
		if err != nil {
			return true
		}
		for _, input := range inputs {
			if int(input.PrevIndex) >= len(unspent.States) || (unspent.States[input.PrevIndex].State&coin) != 0 {
				return true
			}
		}
	}
	return false
}

// Persist flushes all the changes made into the (supposedly) persistent
// underlying store.
func (dao *Simple) Persist() (int, error) {
	return dao.Store.Persist()
}
