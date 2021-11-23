package dao

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	iocore "io"
	"sort"

	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/mpt"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// HasTransaction errors.
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
	DeleteBlock(h util.Uint256, buf *io.BufBinWriter) error
	DeleteContractID(id int32) error
	DeleteStorageItem(id int32, key []byte) error
	GetAndDecode(entity io.Serializable, key []byte) error
	GetAppExecResults(hash util.Uint256, trig trigger.Type) ([]state.AppExecResult, error)
	GetBatch() *storage.MemBatch
	GetBlock(hash util.Uint256) (*block.Block, error)
	GetContractScriptHash(id int32) (util.Uint160, error)
	GetCurrentBlockHeight() (uint32, error)
	GetCurrentHeaderHeight() (i uint32, h util.Uint256, err error)
	GetHeaderHashes() ([]util.Uint256, error)
	GetTokenTransferInfo(acc util.Uint160) (*state.TokenTransferInfo, error)
	GetTokenTransferLog(acc util.Uint160, index uint32, isNEP11 bool) (*state.TokenTransferLog, error)
	GetStateSyncPoint() (uint32, error)
	GetStateSyncCurrentBlockHeight() (uint32, error)
	GetStorageItem(id int32, key []byte) state.StorageItem
	GetStorageItems(id int32) ([]state.StorageItemWithKey, error)
	GetStorageItemsWithPrefix(id int32, prefix []byte) ([]state.StorageItemWithKey, error)
	GetTransaction(hash util.Uint256) (*transaction.Transaction, uint32, error)
	GetVersion() (string, error)
	GetWrapped() DAO
	HasTransaction(hash util.Uint256) error
	Persist() (int, error)
	PersistSync() (int, error)
	PutAppExecResult(aer *state.AppExecResult, buf *io.BufBinWriter) error
	PutContractID(id int32, hash util.Uint160) error
	PutCurrentHeader(hashAndIndex []byte) error
	PutTokenTransferInfo(acc util.Uint160, bs *state.TokenTransferInfo) error
	PutTokenTransferLog(acc util.Uint160, index uint32, isNEP11 bool, lg *state.TokenTransferLog) error
	PutStateSyncPoint(p uint32) error
	PutStateSyncCurrentBlockHeight(h uint32) error
	PutStorageItem(id int32, key []byte, si state.StorageItem) error
	PutVersion(v string) error
	Seek(id int32, prefix []byte, f func(k, v []byte))
	SeekAsync(ctx context.Context, id int32, prefix []byte) chan storage.KeyValue
	StoreAsBlock(block *block.Block, buf *io.BufBinWriter) error
	StoreAsCurrentBlock(block *block.Block, buf *io.BufBinWriter) error
	StoreAsTransaction(tx *transaction.Transaction, index uint32, buf *io.BufBinWriter) error
	putTokenTransferInfo(acc util.Uint160, bs *state.TokenTransferInfo, buf *io.BufBinWriter) error
}

// Simple is memCached wrapper around DB, simple DAO implementation.
type Simple struct {
	Store *storage.MemCachedStore
	// stateRootInHeader specifies if block header contains state root.
	stateRootInHeader bool
	// p2pSigExtensions denotes whether P2PSignatureExtensions are enabled.
	p2pSigExtensions bool
}

// NewSimple creates new simple dao using provided backend store.
func NewSimple(backend storage.Store, stateRootInHeader bool, p2pSigExtensions bool) *Simple {
	st := storage.NewMemCachedStore(backend)
	return &Simple{Store: st, stateRootInHeader: stateRootInHeader, p2pSigExtensions: p2pSigExtensions}
}

// GetBatch returns currently accumulated DB changeset.
func (dao *Simple) GetBatch() *storage.MemBatch {
	return dao.Store.GetBatch()
}

// GetWrapped returns new DAO instance with another layer of wrapped
// MemCachedStore around the current DAO Store.
func (dao *Simple) GetWrapped() DAO {
	d := NewSimple(dao.Store, dao.stateRootInHeader, dao.p2pSigExtensions)
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

func makeContractIDKey(id int32) []byte {
	key := make([]byte, 5)
	key[0] = byte(storage.STContractID)
	binary.LittleEndian.PutUint32(key[1:], uint32(id))
	return key
}

// DeleteContractID deletes contract's id to hash mapping.
func (dao *Simple) DeleteContractID(id int32) error {
	return dao.Store.Delete(makeContractIDKey(id))
}

// PutContractID adds a mapping from contract's ID to its hash.
func (dao *Simple) PutContractID(id int32, hash util.Uint160) error {
	return dao.Store.Put(makeContractIDKey(id), hash.BytesBE())
}

// GetContractScriptHash retrieves contract's hash given its ID.
func (dao *Simple) GetContractScriptHash(id int32) (util.Uint160, error) {
	var data = new(util.Uint160)
	if err := dao.GetAndDecode(data, makeContractIDKey(id)); err != nil {
		return *data, err
	}
	return *data, nil
}

// -- start NEP-17 transfer info.

// GetTokenTransferInfo retrieves NEP-17 transfer info from the cache.
func (dao *Simple) GetTokenTransferInfo(acc util.Uint160) (*state.TokenTransferInfo, error) {
	key := storage.AppendPrefix(storage.STTokenTransferInfo, acc.BytesBE())
	bs := state.NewTokenTransferInfo()
	err := dao.GetAndDecode(bs, key)
	if err != nil && err != storage.ErrKeyNotFound {
		return nil, err
	}
	return bs, nil
}

// PutTokenTransferInfo saves NEP-17 transfer info in the cache.
func (dao *Simple) PutTokenTransferInfo(acc util.Uint160, bs *state.TokenTransferInfo) error {
	return dao.putTokenTransferInfo(acc, bs, io.NewBufBinWriter())
}

func (dao *Simple) putTokenTransferInfo(acc util.Uint160, bs *state.TokenTransferInfo, buf *io.BufBinWriter) error {
	key := storage.AppendPrefix(storage.STTokenTransferInfo, acc.BytesBE())
	return dao.putWithBuffer(bs, key, buf)
}

// -- end NEP-17 transfer info.

// -- start transfer log.

func getTokenTransferLogKey(acc util.Uint160, index uint32, isNEP11 bool) []byte {
	key := make([]byte, 1+util.Uint160Size+4)
	if isNEP11 {
		key[0] = byte(storage.STNEP11Transfers)
	} else {
		key[0] = byte(storage.STNEP17Transfers)
	}
	copy(key[1:], acc.BytesBE())
	binary.LittleEndian.PutUint32(key[util.Uint160Size:], index)
	return key
}

// GetTokenTransferLog retrieves transfer log from the cache.
func (dao *Simple) GetTokenTransferLog(acc util.Uint160, index uint32, isNEP11 bool) (*state.TokenTransferLog, error) {
	key := getTokenTransferLogKey(acc, index, isNEP11)
	value, err := dao.Store.Get(key)
	if err != nil {
		if err == storage.ErrKeyNotFound {
			return new(state.TokenTransferLog), nil
		}
		return nil, err
	}
	return &state.TokenTransferLog{Raw: value}, nil
}

// PutTokenTransferLog saves given transfer log in the cache.
func (dao *Simple) PutTokenTransferLog(acc util.Uint160, index uint32, isNEP11 bool, lg *state.TokenTransferLog) error {
	key := getTokenTransferLogKey(acc, index, isNEP11)
	return dao.Store.Put(key, lg.Raw)
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

// GetStorageItem returns StorageItem if it exists in the given store.
func (dao *Simple) GetStorageItem(id int32, key []byte) state.StorageItem {
	b, err := dao.Store.Get(makeStorageItemKey(id, key))
	if err != nil {
		return nil
	}
	return b
}

// PutStorageItem puts given StorageItem for given id with given
// key into the given store.
func (dao *Simple) PutStorageItem(id int32, key []byte, si state.StorageItem) error {
	stKey := makeStorageItemKey(id, key)
	return dao.Store.Put(stKey, si)
}

// DeleteStorageItem drops storage item for the given id with the
// given key from the store.
func (dao *Simple) DeleteStorageItem(id int32, key []byte) error {
	stKey := makeStorageItemKey(id, key)
	return dao.Store.Delete(stKey)
}

// GetStorageItems returns all storage items for a given id.
func (dao *Simple) GetStorageItems(id int32) ([]state.StorageItemWithKey, error) {
	return dao.GetStorageItemsWithPrefix(id, nil)
}

// GetStorageItemsWithPrefix returns all storage items with given id for a
// given scripthash.
func (dao *Simple) GetStorageItemsWithPrefix(id int32, prefix []byte) ([]state.StorageItemWithKey, error) {
	var siArr []state.StorageItemWithKey

	saveToArr := func(k, v []byte) {
		// Cut prefix and hash.
		// #1468, but don't need to copy here, because it is done by Store.
		siArr = append(siArr, state.StorageItemWithKey{
			Key:  k,
			Item: state.StorageItem(v),
		})
	}
	dao.Seek(id, prefix, saveToArr)
	return siArr, nil
}

// Seek executes f for all items with a given prefix.
// If key is to be used outside of f, they may not be copied.
func (dao *Simple) Seek(id int32, prefix []byte, f func(k, v []byte)) {
	lookupKey := makeStorageItemKey(id, nil)
	if prefix != nil {
		lookupKey = append(lookupKey, prefix...)
	}
	dao.Store.Seek(lookupKey, func(k, v []byte) {
		f(k[len(lookupKey):], v)
	})
}

// SeekAsync sends all storage items matching given prefix to a channel and returns
// the channel. Resulting keys and values may not be copied.
func (dao *Simple) SeekAsync(ctx context.Context, id int32, prefix []byte) chan storage.KeyValue {
	lookupKey := makeStorageItemKey(id, nil)
	if prefix != nil {
		lookupKey = append(lookupKey, prefix...)
	}
	return dao.Store.SeekAsync(ctx, lookupKey, true)
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
	key := storage.AppendPrefix(storage.DataBlock, hash.BytesBE())
	b, err := dao.Store.Get(key)
	if err != nil {
		return nil, err
	}

	block, err := block.NewBlockFromTrimmedBytes(dao.stateRootInHeader, b)
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

// GetStateSyncPoint returns current state synchronisation point P.
func (dao *Simple) GetStateSyncPoint() (uint32, error) {
	b, err := dao.Store.Get(storage.SYSStateSyncPoint.Bytes())
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(b), nil
}

// GetStateSyncCurrentBlockHeight returns current block height stored during state
// synchronisation process.
func (dao *Simple) GetStateSyncCurrentBlockHeight() (uint32, error) {
	b, err := dao.Store.Get(storage.SYSStateSyncCurrentBlockHeight.Bytes())
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(b), nil
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
	key := storage.AppendPrefix(storage.DataTransaction, hash.BytesBE())
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

// PutStateSyncPoint stores current state synchronisation point P.
func (dao *Simple) PutStateSyncPoint(p uint32) error {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, p)
	return dao.Store.Put(storage.SYSStateSyncPoint.Bytes(), buf)
}

// PutStateSyncCurrentBlockHeight stores current block height during state synchronisation process.
func (dao *Simple) PutStateSyncCurrentBlockHeight(h uint32) error {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, h)
	return dao.Store.Put(storage.SYSStateSyncCurrentBlockHeight.Bytes(), buf)
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
	key := storage.AppendPrefix(storage.DataTransaction, hash.BytesBE())
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
		key = storage.AppendPrefix(storage.DataBlock, block.Hash().BytesBE())
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

// DeleteBlock removes block from dao.
func (dao *Simple) DeleteBlock(h util.Uint256, w *io.BufBinWriter) error {
	batch := dao.Store.Batch()
	key := make([]byte, util.Uint256Size+1)
	key[0] = byte(storage.DataBlock)
	copy(key[1:], h.BytesBE())
	bs, err := dao.Store.Get(key)
	if err != nil {
		return err
	}

	b, err := block.NewBlockFromTrimmedBytes(dao.stateRootInHeader, bs)
	if err != nil {
		return err
	}

	if w == nil {
		w = io.NewBufBinWriter()
	}
	b.Header.EncodeBinary(w.BinWriter)
	w.BinWriter.WriteB(0)
	if w.Err != nil {
		return w.Err
	}
	batch.Put(key, w.Bytes())

	key[0] = byte(storage.DataTransaction)
	for _, tx := range b.Transactions {
		copy(key[1:], tx.Hash().BytesBE())
		batch.Delete(key)
		if dao.p2pSigExtensions {
			for _, attr := range tx.GetAttributes(transaction.ConflictsT) {
				hash := attr.Value.(*transaction.Conflicts).Hash
				copy(key[1:], hash.BytesBE())
				batch.Delete(key)
			}
		}
		key[0] = byte(storage.STNotification)
		batch.Delete(key)
	}

	key[0] = byte(storage.STNotification)
	copy(key[1:], h.BytesBE())
	batch.Delete(key)

	return dao.Store.PutBatch(batch)
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

// StoreAsTransaction stores given TX as DataTransaction. It also stores transactions
// given tx has conflicts with as DataTransaction with dummy version. It can reuse given
// buffer for the purpose of value serialization.
func (dao *Simple) StoreAsTransaction(tx *transaction.Transaction, index uint32, buf *io.BufBinWriter) error {
	key := storage.AppendPrefix(storage.DataTransaction, tx.Hash().BytesBE())
	if buf == nil {
		buf = io.NewBufBinWriter()
	}
	buf.WriteU32LE(index)
	tx.EncodeBinary(buf.BinWriter)
	if buf.Err != nil {
		return buf.Err
	}
	err := dao.Store.Put(key, buf.Bytes())
	if err != nil {
		return err
	}
	if dao.p2pSigExtensions {
		var value []byte
		for _, attr := range tx.GetAttributes(transaction.ConflictsT) {
			hash := attr.Value.(*transaction.Conflicts).Hash
			copy(key[1:], hash.BytesBE())
			if value == nil {
				buf.Reset()
				buf.WriteU32LE(index)
				buf.BinWriter.WriteB(transaction.DummyVersion)
				value = buf.Bytes()
			}
			err = dao.Store.Put(key, value)
			if err != nil {
				return fmt.Errorf("failed to store conflicting transaction %s for transaction %s: %w", hash.StringLE(), tx.Hash().StringLE(), err)
			}
		}
	}
	return nil
}

// Persist flushes all the changes made into the (supposedly) persistent
// underlying store. It doesn't block accesses to DAO from other threads.
func (dao *Simple) Persist() (int, error) {
	return dao.Store.Persist()
}

// PersistSync flushes all the changes made into the (supposedly) persistent
// underlying store. It's a synchronous version of Persist that doesn't allow
// other threads to work with DAO while flushing the Store.
func (dao *Simple) PersistSync() (int, error) {
	return dao.Store.PersistSync()
}

// GetMPTBatch storage changes to be applied to MPT.
func (dao *Simple) GetMPTBatch() mpt.Batch {
	var b mpt.Batch
	dao.Store.MemoryStore.SeekAll([]byte{byte(storage.STStorage)}, func(k, v []byte) {
		b.Add(k[1:], v)
	})
	return b
}
