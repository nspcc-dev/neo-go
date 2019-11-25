package core

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sort"

	"github.com/CityOfZion/neo-go/pkg/core/entities"
	"github.com/CityOfZion/neo-go/pkg/core/storage"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/crypto/keys"
	"github.com/CityOfZion/neo-go/pkg/io"
	"github.com/CityOfZion/neo-go/pkg/util"
)

// dao is a data access object.
type dao struct {
	store *storage.MemCachedStore
}

// GetAndDecode performs get operation and decoding with serializable structures.
func (dao *dao) GetAndDecode(entity io.Serializable, key []byte) error {
	entityBytes, err := dao.store.Get(key)
	if err != nil {
		return err
	}
	reader := io.NewBinReaderFromBuf(entityBytes)
	entity.DecodeBinary(reader)
	return reader.Err
}

// Put performs put operation with serializable structures.
func (dao *dao) Put(entity io.Serializable, key []byte) error {
	buf := io.NewBufBinWriter()
	entity.EncodeBinary(buf.BinWriter)
	if buf.Err != nil {
		return buf.Err
	}
	return dao.store.Put(key, buf.Bytes())
}

// -- start accounts.

// GetAccountStateOrNew retrieves AccountState from temporary or persistent Store
// or creates a new one if it doesn't exist and persists it.
func (dao *dao) GetAccountStateOrNew(hash util.Uint160) (*entities.AccountState, error) {
	account, err := dao.GetAccountState(hash)
	if err != nil {
		if err != storage.ErrKeyNotFound {
			return nil, err
		}
		account = entities.NewAccountState(hash)
		if err = dao.PutAccountState(account); err != nil {
			return nil, err
		}
	}
	return account, nil
}

// GetAccountState returns AccountState from the given Store if it's
// present there. Returns nil otherwise.
func (dao *dao) GetAccountState(hash util.Uint160) (*entities.AccountState, error) {
	account := &entities.AccountState{}
	key := storage.AppendPrefix(storage.STAccount, hash.BytesBE())
	err := dao.GetAndDecode(account, key)
	if err != nil {
		return nil, err
	}
	return account, err
}

// PutAccountState puts given AccountState into the given store.
func (dao *dao) PutAccountState(as *entities.AccountState) error {
	key := storage.AppendPrefix(storage.STAccount, as.ScriptHash.BytesBE())
	return dao.Put(as, key)
}

// -- end accounts.

// -- start assets.

// GetAssetState returns given asset state as recorded in the given store.
func (dao *dao) GetAssetState(assetID util.Uint256) (*entities.AssetState, error) {
	asset := &entities.AssetState{}
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
func (dao *dao) PutAssetState(as *entities.AssetState) error {
	key := storage.AppendPrefix(storage.STAsset, as.ID.BytesBE())
	return dao.Put(as, key)
}

// -- end assets.

// -- start contracts.

// GetContractState returns contract state as recorded in the given
// store by the given script hash.
func (dao *dao) GetContractState(hash util.Uint160) (*entities.ContractState, error) {
	contract := &entities.ContractState{}
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
func (dao *dao) PutContractState(cs *entities.ContractState) error {
	key := storage.AppendPrefix(storage.STContract, cs.ScriptHash().BytesBE())
	return dao.Put(cs, key)
}

// DeleteContractState deletes given contract state in the given store.
func (dao *dao) DeleteContractState(hash util.Uint160) error {
	key := storage.AppendPrefix(storage.STContract, hash.BytesBE())
	return dao.store.Delete(key)
}

// -- end contracts.

// -- start unspent coins.

// GetUnspentCoinStateOrNew gets UnspentCoinState from temporary or persistent Store
// and return it. If it's not present in both stores, returns a new
// UnspentCoinState.
func (dao *dao) GetUnspentCoinStateOrNew(hash util.Uint256) (*UnspentCoinState, error) {
	unspent, err := dao.GetUnspentCoinState(hash)
	if err != nil {
		if err != storage.ErrKeyNotFound {
			return nil, err
		}
		unspent = &UnspentCoinState{
			states: []entities.CoinState{},
		}
		if err = dao.PutUnspentCoinState(hash, unspent); err != nil {
			return nil, err
		}
	}
	return unspent, nil
}

// GetUnspentCoinState retrieves UnspentCoinState from the given store.
func (dao *dao) GetUnspentCoinState(hash util.Uint256) (*UnspentCoinState, error) {
	unspent := &UnspentCoinState{}
	key := storage.AppendPrefix(storage.STCoin, hash.BytesLE())
	err := dao.GetAndDecode(unspent, key)
	if err != nil {
		return nil, err
	}
	return unspent, nil
}

// PutUnspentCoinState puts given UnspentCoinState into the given store.
func (dao *dao) PutUnspentCoinState(hash util.Uint256, ucs *UnspentCoinState) error {
	key := storage.AppendPrefix(storage.STCoin, hash.BytesLE())
	return dao.Put(ucs, key)
}

// -- end unspent coins.

// -- start spent coins.

// GetSpentCoinsOrNew returns spent coins from store.
func (dao *dao) GetSpentCoinsOrNew(hash util.Uint256) (*SpentCoinState, error) {
	spent, err := dao.GetSpentCoinState(hash)
	if err != nil {
		if err != storage.ErrKeyNotFound {
			return nil, err
		}
		spent = &SpentCoinState{
			items: make(map[uint16]uint32),
		}
		if err = dao.PutSpentCoinState(hash, spent); err != nil {
			return nil, err
		}
	}
	return spent, nil
}

// GetSpentCoinState gets SpentCoinState from the given store.
func (dao *dao) GetSpentCoinState(hash util.Uint256) (*SpentCoinState, error) {
	spent := &SpentCoinState{}
	key := storage.AppendPrefix(storage.STSpentCoin, hash.BytesLE())
	err := dao.GetAndDecode(spent, key)
	if err != nil {
		return nil, err
	}
	return spent, nil
}

// PutSpentCoinState puts given SpentCoinState into the given store.
func (dao *dao) PutSpentCoinState(hash util.Uint256, scs *SpentCoinState) error {
	key := storage.AppendPrefix(storage.STSpentCoin, hash.BytesLE())
	return dao.Put(scs, key)
}

// DeleteSpentCoinState deletes given SpentCoinState from the given store.
func (dao *dao) DeleteSpentCoinState(hash util.Uint256) error {
	key := storage.AppendPrefix(storage.STSpentCoin, hash.BytesLE())
	return dao.store.Delete(key)
}

// -- end spent coins.

// -- start validator.

// GetValidatorStateOrNew gets validator from store or created new one in case of error.
func (dao *dao) GetValidatorStateOrNew(publicKey *keys.PublicKey) (*entities.ValidatorState, error) {
	validatorState, err := dao.GetValidatorState(publicKey)
	if err != nil {
		if err != storage.ErrKeyNotFound {
			return nil, err
		}
		validatorState = &entities.ValidatorState{PublicKey: publicKey}
		if err = dao.PutValidatorState(validatorState); err != nil {
			return nil, err
		}
	}
	return validatorState, nil

}

// GetValidators returns all validators from store.
func (dao *dao) GetValidators() []*entities.ValidatorState {
	var validators []*entities.ValidatorState
	dao.store.Seek(storage.STValidator.Bytes(), func(k, v []byte) {
		r := io.NewBinReaderFromBuf(v)
		validator := &entities.ValidatorState{}
		validator.DecodeBinary(r)
		if r.Err != nil {
			return
		}
		validators = append(validators, validator)
	})
	return validators
}

// GetValidatorState returns validator by publicKey.
func (dao *dao) GetValidatorState(publicKey *keys.PublicKey) (*entities.ValidatorState, error) {
	validatorState := &entities.ValidatorState{}
	key := storage.AppendPrefix(storage.STValidator, publicKey.Bytes())
	err := dao.GetAndDecode(validatorState, key)
	if err != nil {
		return nil, err
	}
	return validatorState, nil
}

// PutValidatorState puts given ValidatorState into the given store.
func (dao *dao) PutValidatorState(vs *entities.ValidatorState) error {
	key := storage.AppendPrefix(storage.STValidator, vs.PublicKey.Bytes())
	return dao.Put(vs, key)
}

// DeleteValidatorState deletes given ValidatorState into the given store.
func (dao *dao) DeleteValidatorState(vs *entities.ValidatorState) error {
	key := storage.AppendPrefix(storage.STValidator, vs.PublicKey.Bytes())
	return dao.store.Delete(key)
}

// -- end validator.

// -- start notification event.

// GetAppExecResult gets application execution result from the
// given store.
func (dao *dao) GetAppExecResult(hash util.Uint256) (*entities.AppExecResult, error) {
	aer := &entities.AppExecResult{}
	key := storage.AppendPrefix(storage.STNotification, hash.BytesBE())
	err := dao.GetAndDecode(aer, key)
	if err != nil {
		return nil, err
	}
	return aer, nil
}

// PutAppExecResult puts given application execution result into the
// given store.
func (dao *dao) PutAppExecResult(aer *entities.AppExecResult) error {
	key := storage.AppendPrefix(storage.STNotification, aer.TxHash.BytesBE())
	return dao.Put(aer, key)
}

// -- end notification event.

// -- start storage item.

// GetStorageItem returns StorageItem if it exists in the given Store.
func (dao *dao) GetStorageItem(scripthash util.Uint160, key []byte) *entities.StorageItem {
	b, err := dao.store.Get(makeStorageItemKey(scripthash, key))
	if err != nil {
		return nil
	}
	r := io.NewBinReaderFromBuf(b)

	si := &entities.StorageItem{}
	si.DecodeBinary(r)
	if r.Err != nil {
		return nil
	}

	return si
}

// PutStorageItem puts given StorageItem for given script with given
// key into the given Store.
func (dao *dao) PutStorageItem(scripthash util.Uint160, key []byte, si *entities.StorageItem) error {
	return dao.Put(si, makeStorageItemKey(scripthash, key))
}

// DeleteStorageItem drops storage item for the given script with the
// given key from the Store.
func (dao *dao) DeleteStorageItem(scripthash util.Uint160, key []byte) error {
	return dao.store.Delete(makeStorageItemKey(scripthash, key))
}

// GetStorageItems returns all storage items for a given scripthash.
func (dao *dao) GetStorageItems(hash util.Uint160) (map[string]*entities.StorageItem, error) {
	var siMap = make(map[string]*entities.StorageItem)
	var err error

	saveToMap := func(k, v []byte) {
		if err != nil {
			return
		}
		r := io.NewBinReaderFromBuf(v)
		si := &entities.StorageItem{}
		si.DecodeBinary(r)
		if r.Err != nil {
			err = r.Err
			return
		}

		// Cut prefix and hash.
		siMap[string(k[21:])] = si
	}
	dao.store.Seek(storage.AppendPrefix(storage.STStorage, hash.BytesLE()), saveToMap)
	if err != nil {
		return nil, err
	}
	return siMap, nil
}

// makeStorageItemKey returns a key used to store StorageItem in the DB.
func makeStorageItemKey(scripthash util.Uint160, key []byte) []byte {
	return storage.AppendPrefix(storage.STStorage, append(scripthash.BytesLE(), key...))
}

// -- end storage item.

// -- other.

// GetBlock returns Block by the given hash if it exists in the store.
func (dao *dao) GetBlock(hash util.Uint256) (*Block, error) {
	key := storage.AppendPrefix(storage.DataBlock, hash.BytesLE())
	b, err := dao.store.Get(key)
	if err != nil {
		return nil, err
	}
	block, err := NewBlockFromTrimmedBytes(b)
	if err != nil {
		return nil, err
	}
	return block, err
}

// GetVersion attempts to get the current version stored in the
// underlying Store.
func (dao *dao) GetVersion() (string, error) {
	version, err := dao.store.Get(storage.SYSVersion.Bytes())
	return string(version), err
}

// GetCurrentBlockHeight returns the current block height found in the
// underlying Store.
func (dao *dao) GetCurrentBlockHeight() (uint32, error) {
	b, err := dao.store.Get(storage.SYSCurrentBlock.Bytes())
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(b[32:36]), nil
}

// GetCurrentHeaderHeight returns the current header height and hash from
// the underlying Store.
func (dao *dao) GetCurrentHeaderHeight() (i uint32, h util.Uint256, err error) {
	var b []byte
	b, err = dao.store.Get(storage.SYSCurrentHeader.Bytes())
	if err != nil {
		return
	}
	i = binary.LittleEndian.Uint32(b[32:36])
	h, err = util.Uint256DecodeBytesLE(b[:32])
	return
}

// GetHeaderHashes returns a sorted list of header hashes retrieved from
// the given underlying Store.
func (dao *dao) GetHeaderHashes() ([]util.Uint256, error) {
	hashMap := make(map[uint32][]util.Uint256)
	dao.store.Seek(storage.IXHeaderHashList.Bytes(), func(k, v []byte) {
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
	sort.Sort(slice(sortedKeys))

	for _, key := range sortedKeys {
		hashes = append(hashes[:key], hashMap[key]...)
	}

	return hashes, nil
}

// GetTransaction returns Transaction and its height by the given hash
// if it exists in the store.
func (dao *dao) GetTransaction(hash util.Uint256) (*transaction.Transaction, uint32, error) {
	key := storage.AppendPrefix(storage.DataTransaction, hash.BytesLE())
	b, err := dao.store.Get(key)
	if err != nil {
		return nil, 0, err
	}
	r := io.NewBinReaderFromBuf(b)

	var height uint32
	r.ReadLE(&height)

	tx := &transaction.Transaction{}
	tx.DecodeBinary(r)
	if r.Err != nil {
		return nil, 0, r.Err
	}

	return tx, height, nil
}

// PutVersion stores the given version in the underlying Store.
func (dao *dao) PutVersion(v string) error {
	return dao.store.Put(storage.SYSVersion.Bytes(), []byte(v))
}

// PutCurrentHeader stores current header.
func (dao *dao) PutCurrentHeader(hashAndIndex []byte) error {
	return dao.store.Put(storage.SYSCurrentHeader.Bytes(), hashAndIndex)
}

// read2000Uint256Hashes attempts to read 2000 Uint256 hashes from
// the given byte array.
func read2000Uint256Hashes(b []byte) ([]util.Uint256, error) {
	r := bytes.NewReader(b)
	br := io.NewBinReaderFromIO(r)
	lenHashes := br.ReadVarUint()
	hashes := make([]util.Uint256, lenHashes)
	br.ReadLE(hashes)
	if br.Err != nil {
		return nil, br.Err
	}
	return hashes, nil
}

// HasTransaction returns true if the given store contains the given
// Transaction hash.
func (dao *dao) HasTransaction(hash util.Uint256) bool {
	key := storage.AppendPrefix(storage.DataTransaction, hash.BytesLE())
	if _, err := dao.store.Get(key); err == nil {
		return true
	}
	return false
}

// StoreAsBlock stores the given block as DataBlock.
func (dao *dao) StoreAsBlock(block *Block, sysFee uint32) error {
	var (
		key = storage.AppendPrefix(storage.DataBlock, block.Hash().BytesLE())
		buf = io.NewBufBinWriter()
	)
	// sysFee needs to be handled somehow
	//	buf.WriteLE(sysFee)
	b, err := block.Trim()
	if err != nil {
		return err
	}
	buf.WriteLE(b)
	if buf.Err != nil {
		return buf.Err
	}
	return dao.store.Put(key, buf.Bytes())
}

// StoreAsCurrentBlock stores the given block witch prefix SYSCurrentBlock.
func (dao *dao) StoreAsCurrentBlock(block *Block) error {
	buf := io.NewBufBinWriter()
	buf.WriteLE(block.Hash().BytesLE())
	buf.WriteLE(block.Index)
	return dao.store.Put(storage.SYSCurrentBlock.Bytes(), buf.Bytes())
}

// StoreAsTransaction stores the given TX as DataTransaction.
func (dao *dao) StoreAsTransaction(tx *transaction.Transaction, index uint32) error {
	key := storage.AppendPrefix(storage.DataTransaction, tx.Hash().BytesLE())
	buf := io.NewBufBinWriter()
	buf.WriteLE(index)
	tx.EncodeBinary(buf.BinWriter)
	if buf.Err != nil {
		return buf.Err
	}
	return dao.store.Put(key, buf.Bytes())
}

// IsDoubleSpend verifies that the input transactions are not double spent.
func (dao *dao) IsDoubleSpend(tx *transaction.Transaction) bool {
	if len(tx.Inputs) == 0 {
		return false
	}
	for prevHash, inputs := range tx.GroupInputsByPrevHash() {
		unspent, err := dao.GetUnspentCoinState(prevHash)
		if err != nil {
			return false
		}
		for _, input := range inputs {
			if int(input.PrevIndex) >= len(unspent.states) || unspent.states[input.PrevIndex] == entities.CoinStateSpent {
				return true
			}
		}
	}
	return false
}
