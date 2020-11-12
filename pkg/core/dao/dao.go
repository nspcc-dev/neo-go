package dao

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	iocore "io"
	"sort"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/mpt"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// HasTransaction errors
var (
	// ErrAlreadyExists is returned when transaction exists in dao.
	ErrAlreadyExists = errors.New("transaction already exists")
	// ErrHasConflicts is returned when transaction is in the list of conflicting
	// transactions which are already in dao.
	ErrHasConflicts = errors.New("transaction has conflicts")
)

// DAO is a data access object.
type DAO interface {
	AppendAppExecResult(aer *state.AppExecResult, buf *io.BufBinWriter) error
	AppendNEP5Transfer(acc util.Uint160, index uint32, tr *state.NEP5Transfer) (bool, error)
	DeleteContractState(hash util.Uint160) error
	DeleteStorageItem(id int32, key []byte) error
	GetAndDecode(entity io.Serializable, key []byte) error
	GetAppExecResults(hash util.Uint256, trig trigger.Type) ([]state.AppExecResult, error)
	GetBatch() *storage.MemBatch
	GetBlock(hash util.Uint256) (*block.Block, error)
	GetContractState(hash util.Uint160) (*state.Contract, error)
	GetContractScriptHash(id int32) (util.Uint160, error)
	GetCurrentBlockHeight() (uint32, error)
	GetCurrentHeaderHeight() (i uint32, h util.Uint256, err error)
	GetCurrentStateRootHeight() (uint32, error)
	GetHeaderHashes() ([]util.Uint256, error)
	GetNEP5Balances(acc util.Uint160) (*state.NEP5Balances, error)
	GetNEP5TransferLog(acc util.Uint160, index uint32) (*state.NEP5TransferLog, error)
	GetAndUpdateNextContractID() (int32, error)
	GetStateRoot(height uint32) (*state.MPTRootState, error)
	PutStateRoot(root *state.MPTRootState) error
	GetStorageItem(id int32, key []byte) *state.StorageItem
	GetStorageItems(id int32) (map[string]*state.StorageItem, error)
	GetStorageItemsWithPrefix(id int32, prefix []byte) (map[string]*state.StorageItem, error)
	GetTransaction(hash util.Uint256) (*transaction.Transaction, uint32, error)
	GetVersion() (string, error)
	GetWrapped() DAO
	HasTransaction(hash util.Uint256) error
	Persist() (int, error)
	PutAppExecResult(aer *state.AppExecResult, buf *io.BufBinWriter) error
	PutContractState(cs *state.Contract) error
	PutCurrentHeader(hashAndIndex []byte) error
	PutNEP5Balances(acc util.Uint160, bs *state.NEP5Balances) error
	PutNEP5TransferLog(acc util.Uint160, index uint32, lg *state.NEP5TransferLog) error
	PutStorageItem(id int32, key []byte, si *state.StorageItem) error
	PutVersion(v string) error
	Seek(id int32, prefix []byte, f func(k, v []byte))
	StoreAsBlock(block *block.Block, buf *io.BufBinWriter) error
	StoreAsCurrentBlock(block *block.Block, buf *io.BufBinWriter) error
	StoreAsTransaction(tx *transaction.Transaction, index uint32, buf *io.BufBinWriter) error
	putNEP5Balances(acc util.Uint160, bs *state.NEP5Balances, buf *io.BufBinWriter) error
}

// Simple is memCached wrapper around DB, simple DAO implementation.
type Simple struct {
	MPT     *mpt.Trie
	Store   *storage.MemCachedStore
	network netmode.Magic
}

// NewSimple creates new simple dao using provided backend store.
func NewSimple(backend storage.Store, network netmode.Magic) *Simple {
	st := storage.NewMemCachedStore(backend)
	return &Simple{Store: st, network: network, MPT: mpt.NewTrie(nil, st)}
}

// GetBatch returns currently accumulated DB changeset.
func (dao *Simple) GetBatch() *storage.MemBatch {
	return dao.Store.GetBatch()
}

// GetWrapped returns new DAO instance with another layer of wrapped
// MemCachedStore around the current DAO Store.
func (dao *Simple) GetWrapped() DAO {
	d := NewSimple(dao.Store, dao.network)
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
	if err := dao.Put(cs, key); err != nil {
		return err
	}
	return dao.putContractScriptHash(cs)
}

// DeleteContractState deletes given contract state in the given store.
func (dao *Simple) DeleteContractState(hash util.Uint160) error {
	key := storage.AppendPrefix(storage.STContract, hash.BytesBE())
	return dao.Store.Delete(key)
}

// GetAndUpdateNextContractID returns id for the next contract and increases stored ID.
func (dao *Simple) GetAndUpdateNextContractID() (int32, error) {
	var id int32
	key := storage.SYSContractID.Bytes()
	data, err := dao.Store.Get(key)
	if err == nil {
		id = int32(binary.LittleEndian.Uint32(data))
	} else if err != storage.ErrKeyNotFound {
		return 0, err
	}
	data = make([]byte, 4)
	binary.LittleEndian.PutUint32(data, uint32(id+1))
	return id, dao.Store.Put(key, data)
}

// putContractScriptHash puts given contract script hash into the given store.
// It's a private method because it should be used after PutContractState to keep
// ID-Hash pair always up-to-date.
func (dao *Simple) putContractScriptHash(cs *state.Contract) error {
	key := make([]byte, 5)
	key[0] = byte(storage.STContractID)
	binary.LittleEndian.PutUint32(key[1:], uint32(cs.ID))
	return dao.Store.Put(key, cs.ScriptHash().BytesBE())
}

// GetContractScriptHash returns script hash of the contract with the specified ID.
// Contract with the script hash may be destroyed.
func (dao *Simple) GetContractScriptHash(id int32) (util.Uint160, error) {
	key := make([]byte, 5)
	key[0] = byte(storage.STContractID)
	binary.LittleEndian.PutUint32(key[1:], uint32(id))
	data := &util.Uint160{}
	if err := dao.GetAndDecode(data, key); err != nil {
		return *data, err
	}
	return *data, nil
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

func getNEP5TransferLogKey(acc util.Uint160, index uint32) []byte {
	key := make([]byte, 1+util.Uint160Size+4)
	key[0] = byte(storage.STNEP5Transfers)
	copy(key[1:], acc.BytesBE())
	binary.LittleEndian.PutUint32(key[util.Uint160Size:], index)
	return key
}

// GetNEP5TransferLog retrieves transfer log from the cache.
func (dao *Simple) GetNEP5TransferLog(acc util.Uint160, index uint32) (*state.NEP5TransferLog, error) {
	key := getNEP5TransferLogKey(acc, index)
	value, err := dao.Store.Get(key)
	if err != nil {
		if err == storage.ErrKeyNotFound {
			return new(state.NEP5TransferLog), nil
		}
		return nil, err
	}
	return &state.NEP5TransferLog{Raw: value}, nil
}

// PutNEP5TransferLog saves given transfer log in the cache.
func (dao *Simple) PutNEP5TransferLog(acc util.Uint160, index uint32, lg *state.NEP5TransferLog) error {
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
		lg = new(state.NEP5TransferLog)
	}
	if err := lg.Append(tr); err != nil {
		return false, err
	}
	return lg.Size() >= state.NEP5TransferBatchSize, dao.PutNEP5TransferLog(acc, index, lg)
}

// -- end transfer log.

// -- start notification event.

// GetAppExecResults gets application execution results with the specified trigger from the
// given store.
func (dao *Simple) GetAppExecResults(hash util.Uint256, trig trigger.Type) ([]state.AppExecResult, error) {
	key := storage.AppendPrefix(storage.STNotification, hash.BytesBE())
	aers, err := dao.Store.Get(key)
	if err != nil {
		return nil, err
	}
	r := io.NewBinReaderFromBuf(aers)
	result := make([]state.AppExecResult, 0, 2)
	for {
		aer := new(state.AppExecResult)
		aer.DecodeBinary(r)
		if r.Err != nil {
			if r.Err == iocore.EOF {
				break
			}
			return nil, r.Err
		}
		if aer.Trigger&trig != 0 {
			result = append(result, *aer)
		}
	}
	return result, nil
}

// AppendAppExecResult appends given application execution result to the existing
// set of execution results for the corresponding hash. It can reuse given buffer
// for the purpose of value serialization.
func (dao *Simple) AppendAppExecResult(aer *state.AppExecResult, buf *io.BufBinWriter) error {
	key := storage.AppendPrefix(storage.STNotification, aer.Container.BytesBE())
	aers, err := dao.Store.Get(key)
	if err != nil && err != storage.ErrKeyNotFound {
		return err
	}
	if len(aers) == 0 {
		return dao.PutAppExecResult(aer, buf)
	}
	if buf == nil {
		buf = io.NewBufBinWriter()
	}
	aer.EncodeBinary(buf.BinWriter)
	if buf.Err != nil {
		return buf.Err
	}
	aers = append(aers, buf.Bytes()...)
	return dao.Store.Put(key, aers)
}

// PutAppExecResult puts given application execution result into the
// given store. It can reuse given buffer for the purpose of value serialization.
func (dao *Simple) PutAppExecResult(aer *state.AppExecResult, buf *io.BufBinWriter) error {
	key := storage.AppendPrefix(storage.STNotification, aer.Container.BytesBE())
	if buf == nil {
		return dao.Put(aer, key)
	}
	return dao.putWithBuffer(aer, key, buf)
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
func (dao *Simple) InitMPT(height uint32) error {
	if height == 0 {
		dao.MPT = mpt.NewTrie(nil, dao.Store)
		return nil
	}
	r, err := dao.GetStateRoot(height)
	if err != nil {
		return err
	}
	dao.MPT = mpt.NewTrie(mpt.NewHashNode(r.Root), dao.Store)
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
func (dao *Simple) GetStorageItem(id int32, key []byte) *state.StorageItem {
	b, err := dao.Store.Get(makeStorageItemKey(id, key))
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

// PutStorageItem puts given StorageItem for given id with given
// key into the given store.
func (dao *Simple) PutStorageItem(id int32, key []byte, si *state.StorageItem) error {
	stKey := makeStorageItemKey(id, key)
	buf := io.NewBufBinWriter()
	si.EncodeBinary(buf.BinWriter)
	if buf.Err != nil {
		return buf.Err
	}
	v := buf.Bytes()
	if dao.MPT != nil {
		if err := dao.MPT.Put(stKey[1:], v); err != nil && err != mpt.ErrNotFound {
			return err
		}
	}
	return dao.Store.Put(stKey, v)
}

// DeleteStorageItem drops storage item for the given id with the
// given key from the store.
func (dao *Simple) DeleteStorageItem(id int32, key []byte) error {
	stKey := makeStorageItemKey(id, key)
	if dao.MPT != nil {
		if err := dao.MPT.Delete(stKey[1:]); err != nil && err != mpt.ErrNotFound {
			return err
		}
	}
	return dao.Store.Delete(stKey)
}

// GetStorageItems returns all storage items for a given id.
func (dao *Simple) GetStorageItems(id int32) (map[string]*state.StorageItem, error) {
	return dao.GetStorageItemsWithPrefix(id, nil)
}

// GetStorageItemsWithPrefix returns all storage items with given id for a
// given scripthash.
func (dao *Simple) GetStorageItemsWithPrefix(id int32, prefix []byte) (map[string]*state.StorageItem, error) {
	var siMap = make(map[string]*state.StorageItem)
	var err error

	saveToMap := func(k, v []byte) {
		if err != nil {
			return
		}
		r := io.NewBinReaderFromBuf(v)
		si := &state.StorageItem{}
		si.DecodeBinary(r)
		if r.Err != nil {
			err = r.Err
			return
		}
		// Cut prefix and hash.
		// Must copy here, #1468.
		key := make([]byte, len(k))
		copy(key, k)
		siMap[string(key)] = si
	}
	dao.Seek(id, prefix, saveToMap)
	return siMap, nil
}

// Seek executes f for all items with a given prefix.
// If key is to be used outside of f, they must be copied.
func (dao *Simple) Seek(id int32, prefix []byte, f func(k, v []byte)) {
	lookupKey := makeStorageItemKey(id, nil)
	if prefix != nil {
		lookupKey = append(lookupKey, prefix...)
	}
	dao.Store.Seek(lookupKey, func(k, v []byte) {
		f(k[len(lookupKey):], v)
	})
}

// makeStorageItemKey returns a key used to store StorageItem in the DB.
func makeStorageItemKey(id int32, key []byte) []byte {
	// 1 for prefix + 4 for Uint32 + len(key) for key
	buf := make([]byte, 5+len(key))
	buf[0] = byte(storage.STStorage)
	binary.LittleEndian.PutUint32(buf[1:], uint32(id))
	copy(buf[5:], key)
	return buf
}

// -- end storage item.

// -- other.

// GetBlock returns Block by the given hash if it exists in the store.
func (dao *Simple) GetBlock(hash util.Uint256) (*block.Block, error) {
	key := storage.AppendPrefix(storage.DataBlock, hash.BytesLE())
	b, err := dao.Store.Get(key)
	if err != nil {
		return nil, err
	}

	block, err := block.NewBlockFromTrimmedBytes(dao.network, b)
	if err != nil {
		return nil, err
	}
	return block, nil
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
// if it exists in the store. It does not return dummy transactions.
func (dao *Simple) GetTransaction(hash util.Uint256) (*transaction.Transaction, uint32, error) {
	key := storage.AppendPrefix(storage.DataTransaction, hash.BytesLE())
	b, err := dao.Store.Get(key)
	if err != nil {
		return nil, 0, err
	}
	if len(b) < 5 {
		return nil, 0, errors.New("bad transaction bytes")
	}
	if b[4] == transaction.DummyVersion {
		return nil, 0, storage.ErrKeyNotFound
	}
	r := io.NewBinReaderFromBuf(b)

	var height = r.ReadU32LE()

	tx := &transaction.Transaction{Network: dao.network}
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

// HasTransaction returns nil if the given store does not contain the given
// Transaction hash. It returns an error in case if transaction is in chain
// or in the list of conflicting transactions.
func (dao *Simple) HasTransaction(hash util.Uint256) error {
	key := storage.AppendPrefix(storage.DataTransaction, hash.BytesLE())
	bytes, err := dao.Store.Get(key)
	if err != nil {
		return nil
	}

	if len(bytes) < 5 {
		return nil
	}
	if bytes[4] == transaction.DummyVersion {
		return ErrHasConflicts
	}
	return ErrAlreadyExists
}

// StoreAsBlock stores given block as DataBlock. It can reuse given buffer for
// the purpose of value serialization.
func (dao *Simple) StoreAsBlock(block *block.Block, buf *io.BufBinWriter) error {
	var (
		key = storage.AppendPrefix(storage.DataBlock, block.Hash().BytesLE())
	)
	if buf == nil {
		buf = io.NewBufBinWriter()
	}
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

// StoreAsCurrentBlock stores a hash of the given block with prefix
// SYSCurrentBlock. It can reuse given buffer for the purpose of value
// serialization.
func (dao *Simple) StoreAsCurrentBlock(block *block.Block, buf *io.BufBinWriter) error {
	if buf == nil {
		buf = io.NewBufBinWriter()
	}
	h := block.Hash()
	h.EncodeBinary(buf.BinWriter)
	buf.WriteU32LE(block.Index)
	return dao.Store.Put(storage.SYSCurrentBlock.Bytes(), buf.Bytes())
}

// StoreAsTransaction stores given TX as DataTransaction. It can reuse given
// buffer for the purpose of value serialization.
func (dao *Simple) StoreAsTransaction(tx *transaction.Transaction, index uint32, buf *io.BufBinWriter) error {
	key := storage.AppendPrefix(storage.DataTransaction, tx.Hash().BytesLE())
	if buf == nil {
		buf = io.NewBufBinWriter()
	}
	buf.WriteU32LE(index)
	if tx.Version == transaction.DummyVersion {
		buf.BinWriter.WriteB(tx.Version)
	} else {
		tx.EncodeBinary(buf.BinWriter)
	}
	if buf.Err != nil {
		return buf.Err
	}
	return dao.Store.Put(key, buf.Bytes())
}

// Persist flushes all the changes made into the (supposedly) persistent
// underlying store.
func (dao *Simple) Persist() (int, error) {
	return dao.Store.Persist()
}
