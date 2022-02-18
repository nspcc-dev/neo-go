package dao

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	iocore "io"
	"sort"

	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/util/slice"
)

// HasTransaction errors.
var (
	// ErrAlreadyExists is returned when transaction exists in dao.
	ErrAlreadyExists = errors.New("transaction already exists")
	// ErrHasConflicts is returned when transaction is in the list of conflicting
	// transactions which are already in dao.
	ErrHasConflicts = errors.New("transaction has conflicts")
)

// Simple is memCached wrapper around DB, simple DAO implementation.
type Simple struct {
	Version Version
	Store   *storage.MemCachedStore
	keyBuf  []byte
	dataBuf *io.BufBinWriter
}

// NewSimple creates new simple dao using provided backend store.
func NewSimple(backend storage.Store, stateRootInHeader bool, p2pSigExtensions bool) *Simple {
	st := storage.NewMemCachedStore(backend)
	return newSimple(st, stateRootInHeader, p2pSigExtensions)
}

func newSimple(st *storage.MemCachedStore, stateRootInHeader bool, p2pSigExtensions bool) *Simple {
	return &Simple{
		Version: Version{
			StoragePrefix:     storage.STStorage,
			StateRootInHeader: stateRootInHeader,
			P2PSigExtensions:  p2pSigExtensions,
		},
		Store: st,
	}
}

// GetBatch returns currently accumulated DB changeset.
func (dao *Simple) GetBatch() *storage.MemBatch {
	return dao.Store.GetBatch()
}

// GetWrapped returns new DAO instance with another layer of wrapped
// MemCachedStore around the current DAO Store.
func (dao *Simple) GetWrapped() *Simple {
	d := NewSimple(dao.Store, dao.Version.StateRootInHeader, dao.Version.P2PSigExtensions)
	d.Version = dao.Version
	return d
}

// GetPrivate returns new DAO instance with another layer of private
// MemCachedStore around the current DAO Store.
func (dao *Simple) GetPrivate() *Simple {
	d := &Simple{}
	*d = *dao                                             // Inherit everything...
	d.Store = storage.NewPrivateMemCachedStore(dao.Store) // except storage, wrap another layer.
	if d.keyBuf == nil {
		d.keyBuf = make([]byte, 0, 1+4+storage.MaxStorageKeyLen) // Prefix, uint32, key.
	}
	if dao.dataBuf == nil {
		d.dataBuf = io.NewBufBinWriter()
	}
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

// putWithBuffer performs put operation using buf as a pre-allocated buffer for serialization.
func (dao *Simple) putWithBuffer(entity io.Serializable, key []byte, buf *io.BufBinWriter) error {
	entity.EncodeBinary(buf.BinWriter)
	if buf.Err != nil {
		return buf.Err
	}
	dao.Store.Put(key, buf.Bytes())
	return nil
}

func (dao *Simple) makeContractIDKey(id int32) []byte {
	key := dao.getKeyBuf(5)
	key[0] = byte(storage.STContractID)
	binary.LittleEndian.PutUint32(key[1:], uint32(id))
	return key
}

// DeleteContractID deletes contract's id to hash mapping.
func (dao *Simple) DeleteContractID(id int32) {
	dao.Store.Delete(dao.makeContractIDKey(id))
}

// PutContractID adds a mapping from contract's ID to its hash.
func (dao *Simple) PutContractID(id int32, hash util.Uint160) {
	dao.Store.Put(dao.makeContractIDKey(id), hash.BytesBE())
}

// GetContractScriptHash retrieves contract's hash given its ID.
func (dao *Simple) GetContractScriptHash(id int32) (util.Uint160, error) {
	var data = new(util.Uint160)
	if err := dao.GetAndDecode(data, dao.makeContractIDKey(id)); err != nil {
		return *data, err
	}
	return *data, nil
}

// -- start NEP-17 transfer info.

func (dao *Simple) makeTTIKey(acc util.Uint160) []byte {
	key := dao.getKeyBuf(1 + util.Uint160Size)
	key[0] = byte(storage.STTokenTransferInfo)
	copy(key[1:], acc.BytesBE())
	return key
}

// GetTokenTransferInfo retrieves NEP-17 transfer info from the cache.
func (dao *Simple) GetTokenTransferInfo(acc util.Uint160) (*state.TokenTransferInfo, error) {
	key := dao.makeTTIKey(acc)
	bs := state.NewTokenTransferInfo()
	err := dao.GetAndDecode(bs, key)
	if err != nil && err != storage.ErrKeyNotFound {
		return nil, err
	}
	return bs, nil
}

// PutTokenTransferInfo saves NEP-17 transfer info in the cache.
func (dao *Simple) PutTokenTransferInfo(acc util.Uint160, bs *state.TokenTransferInfo) error {
	return dao.putTokenTransferInfo(acc, bs, dao.getDataBuf())
}

func (dao *Simple) putTokenTransferInfo(acc util.Uint160, bs *state.TokenTransferInfo, buf *io.BufBinWriter) error {
	return dao.putWithBuffer(bs, dao.makeTTIKey(acc), buf)
}

// -- end NEP-17 transfer info.

// -- start transfer log.

func (dao *Simple) getTokenTransferLogKey(acc util.Uint160, newestTimestamp uint64, index uint32, isNEP11 bool) []byte {
	key := dao.getKeyBuf(1 + util.Uint160Size + 8 + 4)
	if isNEP11 {
		key[0] = byte(storage.STNEP11Transfers)
	} else {
		key[0] = byte(storage.STNEP17Transfers)
	}
	copy(key[1:], acc.BytesBE())
	binary.BigEndian.PutUint64(key[1+util.Uint160Size:], newestTimestamp)
	binary.BigEndian.PutUint32(key[1+util.Uint160Size+8:], index)
	return key
}

// SeekNEP17TransferLog executes f for each NEP-17 transfer in log starting from
// the transfer with the newest timestamp up to the oldest transfer. It continues
// iteration until false is returned from f. The last non-nil error is returned.
func (dao *Simple) SeekNEP17TransferLog(acc util.Uint160, newestTimestamp uint64, f func(*state.NEP17Transfer) (bool, error)) error {
	key := dao.getTokenTransferLogKey(acc, newestTimestamp, 0, false)
	prefixLen := 1 + util.Uint160Size
	var seekErr error
	dao.Store.Seek(storage.SeekRange{
		Prefix:    key[:prefixLen],
		Start:     key[prefixLen : prefixLen+8],
		Backwards: true,
	}, func(k, v []byte) bool {
		lg := &state.TokenTransferLog{Raw: v}
		cont, err := lg.ForEachNEP17(f)
		if err != nil {
			seekErr = err
		}
		return cont
	})
	return seekErr
}

// SeekNEP11TransferLog executes f for each NEP-11 transfer in log starting from
// the transfer with the newest timestamp up to the oldest transfer. It continues
// iteration until false is returned from f. The last non-nil error is returned.
func (dao *Simple) SeekNEP11TransferLog(acc util.Uint160, newestTimestamp uint64, f func(*state.NEP11Transfer) (bool, error)) error {
	key := dao.getTokenTransferLogKey(acc, newestTimestamp, 0, true)
	prefixLen := 1 + util.Uint160Size
	var seekErr error
	dao.Store.Seek(storage.SeekRange{
		Prefix:    key[:prefixLen],
		Start:     key[prefixLen : prefixLen+8],
		Backwards: true,
	}, func(k, v []byte) bool {
		lg := &state.TokenTransferLog{Raw: v}
		cont, err := lg.ForEachNEP11(f)
		if err != nil {
			seekErr = err
		}
		return cont
	})
	return seekErr
}

// GetTokenTransferLog retrieves transfer log from the cache.
func (dao *Simple) GetTokenTransferLog(acc util.Uint160, newestTimestamp uint64, index uint32, isNEP11 bool) (*state.TokenTransferLog, error) {
	key := dao.getTokenTransferLogKey(acc, newestTimestamp, index, isNEP11)
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
func (dao *Simple) PutTokenTransferLog(acc util.Uint160, start uint64, index uint32, isNEP11 bool, lg *state.TokenTransferLog) {
	key := dao.getTokenTransferLogKey(acc, start, index, isNEP11)
	dao.Store.Put(key, lg.Raw)
}

// -- end transfer log.

// -- start notification event.

func (dao *Simple) makeExecutableKey(hash util.Uint256) []byte {
	key := dao.getKeyBuf(1 + util.Uint256Size)
	key[0] = byte(storage.DataExecutable)
	copy(key[1:], hash.BytesBE())
	return key
}

// GetAppExecResults gets application execution results with the specified trigger from the
// given store.
func (dao *Simple) GetAppExecResults(hash util.Uint256, trig trigger.Type) ([]state.AppExecResult, error) {
	key := dao.makeExecutableKey(hash)
	bs, err := dao.Store.Get(key)
	if err != nil {
		return nil, err
	}
	r := io.NewBinReaderFromBuf(bs)
	switch r.ReadB() {
	case storage.ExecBlock:
		_, err = block.NewTrimmedFromReader(dao.Version.StateRootInHeader, r)
		if err != nil {
			return nil, err
		}
	case storage.ExecTransaction:
		_ = r.ReadU32LE()
		tx := &transaction.Transaction{}
		tx.DecodeBinary(r)
	}
	if r.Err != nil {
		return nil, r.Err
	}
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

// -- end notification event.

// -- start storage item.

// GetStorageItem returns StorageItem if it exists in the given store.
func (dao *Simple) GetStorageItem(id int32, key []byte) state.StorageItem {
	b, err := dao.Store.Get(dao.makeStorageItemKey(id, key))
	if err != nil {
		return nil
	}
	return b
}

// PutStorageItem puts given StorageItem for given id with given
// key into the given store.
func (dao *Simple) PutStorageItem(id int32, key []byte, si state.StorageItem) {
	stKey := dao.makeStorageItemKey(id, key)
	dao.Store.Put(stKey, si)
}

// DeleteStorageItem drops storage item for the given id with the
// given key from the store.
func (dao *Simple) DeleteStorageItem(id int32, key []byte) {
	stKey := dao.makeStorageItemKey(id, key)
	dao.Store.Delete(stKey)
}

// GetStorageItems returns all storage items for a given id.
func (dao *Simple) GetStorageItems(id int32) ([]state.StorageItemWithKey, error) {
	return dao.GetStorageItemsWithPrefix(id, nil)
}

// GetStorageItemsWithPrefix returns all storage items with given id for a
// given scripthash.
func (dao *Simple) GetStorageItemsWithPrefix(id int32, prefix []byte) ([]state.StorageItemWithKey, error) {
	var siArr []state.StorageItemWithKey

	saveToArr := func(k, v []byte) bool {
		// Cut prefix and hash.
		// #1468, but don't need to copy here, because it is done by Store.
		siArr = append(siArr, state.StorageItemWithKey{
			Key:  k,
			Item: state.StorageItem(v),
		})
		return true
	}
	dao.Seek(id, storage.SeekRange{Prefix: prefix}, saveToArr)
	return siArr, nil
}

// Seek executes f for all storage items matching a given `rng` (matching given prefix and
// starting from the point specified). If key or value is to be used outside of f, they
// may not be copied. Seek continues iterating until false is returned from f.
func (dao *Simple) Seek(id int32, rng storage.SeekRange, f func(k, v []byte) bool) {
	rng.Prefix = slice.Copy(dao.makeStorageItemKey(id, rng.Prefix)) // f() can use dao too.
	dao.Store.Seek(rng, func(k, v []byte) bool {
		return f(k[len(rng.Prefix):], v)
	})
}

// SeekAsync sends all storage items matching a given `rng` (matching given prefix and
// starting from the point specified) to a channel and returns the channel.
// Resulting keys and values may not be copied.
func (dao *Simple) SeekAsync(ctx context.Context, id int32, rng storage.SeekRange) chan storage.KeyValue {
	rng.Prefix = slice.Copy(dao.makeStorageItemKey(id, rng.Prefix))
	return dao.Store.SeekAsync(ctx, rng, true)
}

// makeStorageItemKey returns a key used to store StorageItem in the DB.
func (dao *Simple) makeStorageItemKey(id int32, key []byte) []byte {
	// 1 for prefix + 4 for Uint32 + len(key) for key
	buf := dao.getKeyBuf(5 + len(key))
	buf[0] = byte(dao.Version.StoragePrefix)
	binary.LittleEndian.PutUint32(buf[1:], uint32(id))
	copy(buf[5:], key)
	return buf
}

// -- end storage item.

// -- other.

// GetBlock returns Block by the given hash if it exists in the store.
func (dao *Simple) GetBlock(hash util.Uint256) (*block.Block, error) {
	return dao.getBlock(dao.makeExecutableKey(hash))
}

func (dao *Simple) getBlock(key []byte) (*block.Block, error) {
	b, err := dao.Store.Get(key)
	if err != nil {
		return nil, err
	}

	r := io.NewBinReaderFromBuf(b)
	if r.ReadB() != storage.ExecBlock {
		return nil, errors.New("internal DB inconsistency")
	}
	block, err := block.NewTrimmedFromReader(dao.Version.StateRootInHeader, r)
	if err != nil {
		return nil, err
	}
	return block, nil
}

// Version represents current dao version.
type Version struct {
	StoragePrefix              storage.KeyPrefix
	StateRootInHeader          bool
	P2PSigExtensions           bool
	P2PStateExchangeExtensions bool
	KeepOnlyLatestState        bool
	Value                      string
}

const (
	stateRootInHeaderBit = 1 << iota
	p2pSigExtensionsBit
	p2pStateExchangeExtensionsBit
	keepOnlyLatestStateBit
)

// FromBytes decodes v from a byte-slice.
func (v *Version) FromBytes(data []byte) error {
	if len(data) == 0 {
		return errors.New("missing version")
	}
	i := 0
	for ; i < len(data) && data[i] != '\x00'; i++ {
	}

	if i == len(data) {
		v.Value = string(data)
		return nil
	}

	if len(data) != i+3 {
		return errors.New("version is invalid")
	}

	v.Value = string(data[:i])
	v.StoragePrefix = storage.KeyPrefix(data[i+1])
	v.StateRootInHeader = data[i+2]&stateRootInHeaderBit != 0
	v.P2PSigExtensions = data[i+2]&p2pSigExtensionsBit != 0
	v.P2PStateExchangeExtensions = data[i+2]&p2pStateExchangeExtensionsBit != 0
	v.KeepOnlyLatestState = data[i+2]&keepOnlyLatestStateBit != 0
	return nil
}

// Bytes encodes v to a byte-slice.
func (v *Version) Bytes() []byte {
	var mask byte
	if v.StateRootInHeader {
		mask |= stateRootInHeaderBit
	}
	if v.P2PSigExtensions {
		mask |= p2pSigExtensionsBit
	}
	if v.P2PStateExchangeExtensions {
		mask |= p2pStateExchangeExtensionsBit
	}
	if v.KeepOnlyLatestState {
		mask |= keepOnlyLatestStateBit
	}
	return append([]byte(v.Value), '\x00', byte(v.StoragePrefix), mask)
}

// GetVersion attempts to get the current version stored in the
// underlying store.
func (dao *Simple) GetVersion() (Version, error) {
	var version Version

	data, err := dao.Store.Get(storage.SYSVersion.Bytes())
	if err == nil {
		err = version.FromBytes(data)
	}
	return version, err
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
	dao.Store.Seek(storage.SeekRange{
		Prefix: storage.IXHeaderHashList.Bytes(),
	}, func(k, v []byte) bool {
		storedCount := binary.LittleEndian.Uint32(k[1:])
		hashes, err := read2000Uint256Hashes(v)
		if err != nil {
			panic(err)
		}
		hashMap[storedCount] = hashes
		return true
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
	key := dao.makeExecutableKey(hash)
	b, err := dao.Store.Get(key)
	if err != nil {
		return nil, 0, err
	}
	if len(b) < 6 {
		return nil, 0, errors.New("bad transaction bytes")
	}
	if b[0] != storage.ExecTransaction {
		return nil, 0, errors.New("internal DB inconsistency")
	}
	if b[5] == transaction.DummyVersion {
		return nil, 0, storage.ErrKeyNotFound
	}
	r := io.NewBinReaderFromBuf(b)
	_ = r.ReadB()

	var height = r.ReadU32LE()

	tx := &transaction.Transaction{}
	tx.DecodeBinary(r)
	if r.Err != nil {
		return nil, 0, r.Err
	}

	return tx, height, nil
}

// PutVersion stores the given version in the underlying store.
func (dao *Simple) PutVersion(v Version) {
	dao.Version = v
	dao.Store.Put(storage.SYSVersion.Bytes(), v.Bytes())
}

// PutCurrentHeader stores current header.
func (dao *Simple) PutCurrentHeader(hashAndIndex []byte) {
	dao.Store.Put(storage.SYSCurrentHeader.Bytes(), hashAndIndex)
}

// PutStateSyncPoint stores current state synchronisation point P.
func (dao *Simple) PutStateSyncPoint(p uint32) {
	buf := dao.getKeyBuf(4) // It's very small, no point in using BufBinWriter.
	binary.LittleEndian.PutUint32(buf, p)
	dao.Store.Put(storage.SYSStateSyncPoint.Bytes(), buf)
}

// PutStateSyncCurrentBlockHeight stores current block height during state synchronisation process.
func (dao *Simple) PutStateSyncCurrentBlockHeight(h uint32) {
	buf := dao.getKeyBuf(4) // It's very small, no point in using BufBinWriter.
	binary.LittleEndian.PutUint32(buf, h)
	dao.Store.Put(storage.SYSStateSyncCurrentBlockHeight.Bytes(), buf)
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
	key := dao.makeExecutableKey(hash)
	bytes, err := dao.Store.Get(key)
	if err != nil {
		return nil
	}

	if len(bytes) < 6 {
		return nil
	}
	if bytes[5] == transaction.DummyVersion {
		return ErrHasConflicts
	}
	return ErrAlreadyExists
}

// StoreAsBlock stores given block as DataBlock. It can reuse given buffer for
// the purpose of value serialization.
func (dao *Simple) StoreAsBlock(block *block.Block, aer1 *state.AppExecResult, aer2 *state.AppExecResult) error {
	var (
		key = dao.makeExecutableKey(block.Hash())
		buf = dao.getDataBuf()
	)
	buf.WriteB(storage.ExecBlock)
	b, err := block.Trim()
	if err != nil {
		return err
	}
	buf.WriteBytes(b)
	if aer1 != nil {
		aer1.EncodeBinary(buf.BinWriter)
	}
	if aer2 != nil {
		aer2.EncodeBinary(buf.BinWriter)
	}
	if buf.Err != nil {
		return buf.Err
	}
	dao.Store.Put(key, buf.Bytes())
	return nil
}

// DeleteBlock removes block from dao. It's not atomic, so make sure you're
// using private MemCached instance here.
func (dao *Simple) DeleteBlock(h util.Uint256) error {
	key := dao.makeExecutableKey(h)

	b, err := dao.getBlock(key)
	if err != nil {
		return err
	}

	err = dao.storeHeader(key, &b.Header)
	if err != nil {
		return err
	}

	for _, tx := range b.Transactions {
		copy(key[1:], tx.Hash().BytesBE())
		dao.Store.Delete(key)
		if dao.Version.P2PSigExtensions {
			for _, attr := range tx.GetAttributes(transaction.ConflictsT) {
				hash := attr.Value.(*transaction.Conflicts).Hash
				copy(key[1:], hash.BytesBE())
				dao.Store.Delete(key)
			}
		}
	}

	return nil
}

// StoreHeader saves block header into the store.
func (dao *Simple) StoreHeader(h *block.Header) error {
	return dao.storeHeader(dao.makeExecutableKey(h.Hash()), h)
}

func (dao *Simple) storeHeader(key []byte, h *block.Header) error {
	buf := dao.getDataBuf()
	buf.WriteB(storage.ExecBlock)
	h.EncodeBinary(buf.BinWriter)
	buf.BinWriter.WriteB(0)
	if buf.Err != nil {
		return buf.Err
	}
	dao.Store.Put(key, buf.Bytes())
	return nil
}

// StoreAsCurrentBlock stores a hash of the given block with prefix
// SYSCurrentBlock. It can reuse given buffer for the purpose of value
// serialization.
func (dao *Simple) StoreAsCurrentBlock(block *block.Block) {
	buf := dao.getDataBuf()
	h := block.Hash()
	h.EncodeBinary(buf.BinWriter)
	buf.WriteU32LE(block.Index)
	dao.Store.Put(storage.SYSCurrentBlock.Bytes(), buf.Bytes())
}

// StoreAsTransaction stores given TX as DataTransaction. It also stores transactions
// given tx has conflicts with as DataTransaction with dummy version. It can reuse given
// buffer for the purpose of value serialization.
func (dao *Simple) StoreAsTransaction(tx *transaction.Transaction, index uint32, aer *state.AppExecResult) error {
	key := dao.makeExecutableKey(tx.Hash())
	buf := dao.getDataBuf()

	buf.WriteB(storage.ExecTransaction)
	buf.WriteU32LE(index)
	tx.EncodeBinary(buf.BinWriter)
	if aer != nil {
		aer.EncodeBinary(buf.BinWriter)
	}
	if buf.Err != nil {
		return buf.Err
	}
	dao.Store.Put(key, buf.Bytes())
	if dao.Version.P2PSigExtensions {
		var value []byte
		for _, attr := range tx.GetAttributes(transaction.ConflictsT) {
			hash := attr.Value.(*transaction.Conflicts).Hash
			copy(key[1:], hash.BytesBE())
			if value == nil {
				buf.Reset()
				buf.WriteB(storage.ExecTransaction)
				buf.WriteU32LE(index)
				buf.BinWriter.WriteB(transaction.DummyVersion)
				value = buf.Bytes()
			}
			dao.Store.Put(key, value)
		}
	}
	return nil
}

func (dao *Simple) getKeyBuf(len int) []byte {
	if dao.keyBuf != nil { // Private DAO.
		return dao.keyBuf[:len] // Should have enough capacity.
	}
	return make([]byte, len)
}

func (dao *Simple) getDataBuf() *io.BufBinWriter {
	if dao.dataBuf != nil {
		dao.dataBuf.Reset()
		return dao.dataBuf
	}
	return io.NewBufBinWriter()
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
