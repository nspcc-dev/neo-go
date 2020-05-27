package dao

import (
	"bytes"
	"errors"
	"sort"

	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// Cached is a data access object that mimics DAO, but has a write cache
// for accounts and read cache for contracts. These are the most frequently used
// objects in the storeBlock().
type Cached struct {
	DAO
	accounts  map[util.Uint160]*state.Account
	contracts map[util.Uint160]*state.Contract
	unspents  map[util.Uint256]*state.UnspentCoin
	balances  map[util.Uint160]*state.NEP5Balances
	transfers map[util.Uint160]map[uint32]*state.NEP5TransferLog
	storage   *itemCache
}

// NewCached returns new Cached wrapping around given backing store.
func NewCached(d DAO) *Cached {
	accs := make(map[util.Uint160]*state.Account)
	ctrs := make(map[util.Uint160]*state.Contract)
	unspents := make(map[util.Uint256]*state.UnspentCoin)
	balances := make(map[util.Uint160]*state.NEP5Balances)
	transfers := make(map[util.Uint160]map[uint32]*state.NEP5TransferLog)
	st := newItemCache()
	dao := d.GetWrapped()
	if cd, ok := dao.(*Cached); ok {
		for h, m := range cd.storage.st {
			for _, k := range cd.storage.keys[h] {
				st.put(h, []byte(k), m[k].State, copyItem(&m[k].StorageItem))
			}
		}
	}
	return &Cached{dao, accs, ctrs, unspents, balances, transfers, st}
}

// GetAccountStateOrNew retrieves Account from cache or underlying store
// or creates a new one if it doesn't exist.
func (cd *Cached) GetAccountStateOrNew(hash util.Uint160) (*state.Account, error) {
	if cd.accounts[hash] != nil {
		return cd.accounts[hash], nil
	}
	return cd.DAO.GetAccountStateOrNew(hash)
}

// GetAccountState retrieves Account from cache or underlying store.
func (cd *Cached) GetAccountState(hash util.Uint160) (*state.Account, error) {
	if cd.accounts[hash] != nil {
		return cd.accounts[hash], nil
	}
	return cd.DAO.GetAccountState(hash)
}

// PutAccountState saves given Account in the cache.
func (cd *Cached) PutAccountState(as *state.Account) error {
	cd.accounts[as.ScriptHash] = as
	return nil
}

// GetContractState returns contract state from cache or underlying store.
func (cd *Cached) GetContractState(hash util.Uint160) (*state.Contract, error) {
	if cd.contracts[hash] != nil {
		return cd.contracts[hash], nil
	}
	cs, err := cd.DAO.GetContractState(hash)
	if err == nil {
		cd.contracts[hash] = cs
	}
	return cs, err
}

// PutContractState puts given contract state into the given store.
func (cd *Cached) PutContractState(cs *state.Contract) error {
	cd.contracts[cs.ScriptHash()] = cs
	return cd.DAO.PutContractState(cs)
}

// DeleteContractState deletes given contract state in cache and backing store.
func (cd *Cached) DeleteContractState(hash util.Uint160) error {
	cd.contracts[hash] = nil
	return cd.DAO.DeleteContractState(hash)
}

// GetUnspentCoinState retrieves UnspentCoin from cache or underlying store.
func (cd *Cached) GetUnspentCoinState(hash util.Uint256) (*state.UnspentCoin, error) {
	if cd.unspents[hash] != nil {
		return cd.unspents[hash], nil
	}
	return cd.DAO.GetUnspentCoinState(hash)
}

// PutUnspentCoinState saves given UnspentCoin in the cache.
func (cd *Cached) PutUnspentCoinState(hash util.Uint256, ucs *state.UnspentCoin) error {
	cd.unspents[hash] = ucs
	return nil
}

// GetNEP5Balances retrieves NEP5Balances for the acc.
func (cd *Cached) GetNEP5Balances(acc util.Uint160) (*state.NEP5Balances, error) {
	if bs := cd.balances[acc]; bs != nil {
		return bs, nil
	}
	return cd.DAO.GetNEP5Balances(acc)
}

// PutNEP5Balances saves NEP5Balances for the acc.
func (cd *Cached) PutNEP5Balances(acc util.Uint160, bs *state.NEP5Balances) error {
	cd.balances[acc] = bs
	return nil
}

// GetNEP5TransferLog retrieves NEP5TransferLog for the acc.
func (cd *Cached) GetNEP5TransferLog(acc util.Uint160, index uint32) (*state.NEP5TransferLog, error) {
	ts := cd.transfers[acc]
	if ts != nil && ts[index] != nil {
		return ts[index], nil
	}
	return cd.DAO.GetNEP5TransferLog(acc, index)
}

// PutNEP5TransferLog saves NEP5TransferLog for the acc.
func (cd *Cached) PutNEP5TransferLog(acc util.Uint160, index uint32, bs *state.NEP5TransferLog) error {
	ts := cd.transfers[acc]
	if ts == nil {
		ts = make(map[uint32]*state.NEP5TransferLog, 2)
		cd.transfers[acc] = ts
	}
	ts[index] = bs
	return nil
}

// AppendNEP5Transfer appends new transfer to a transfer event log.
func (cd *Cached) AppendNEP5Transfer(acc util.Uint160, index uint32, tr *state.NEP5Transfer) (bool, error) {
	lg, err := cd.GetNEP5TransferLog(acc, index)
	if err != nil {
		return false, err
	}
	if err := lg.Append(tr); err != nil {
		return false, err
	}
	return lg.Size() >= nep5TransferBatchSize, cd.PutNEP5TransferLog(acc, index, lg)
}

// Persist flushes all the changes made into the (supposedly) persistent
// underlying store.
func (cd *Cached) Persist() (int, error) {
	if err := cd.FlushStorage(); err != nil {
		return 0, err
	}

	lowerCache, ok := cd.DAO.(*Cached)
	// If the lower DAO is Cached, we only need to flush the MemCached DB.
	// This actually breaks DAO interface incapsulation, but for our current
	// usage scenario it should be good enough if cd doesn't modify object
	// caches (accounts/contracts/etc) in any way.
	if ok {
		var simpleCache *Simple
		for simpleCache == nil {
			if err := lowerCache.FlushStorage(); err != nil {
				return 0, err
			}
			simpleCache, ok = lowerCache.DAO.(*Simple)
			if !ok {
				lowerCache, ok = cd.DAO.(*Cached)
				if !ok {
					return 0, errors.New("unsupported lower DAO")
				}
			}
		}
		return simpleCache.Persist()
	}
	buf := io.NewBufBinWriter()

	for sc := range cd.accounts {
		err := cd.DAO.putAccountState(cd.accounts[sc], buf)
		if err != nil {
			return 0, err
		}
		buf.Reset()
	}
	for hash := range cd.unspents {
		err := cd.DAO.putUnspentCoinState(hash, cd.unspents[hash], buf)
		if err != nil {
			return 0, err
		}
		buf.Reset()
	}
	for acc, bs := range cd.balances {
		err := cd.DAO.putNEP5Balances(acc, bs, buf)
		if err != nil {
			return 0, err
		}
		buf.Reset()
	}
	for acc, ts := range cd.transfers {
		for ind, lg := range ts {
			err := cd.DAO.PutNEP5TransferLog(acc, ind, lg)
			if err != nil {
				return 0, err
			}
		}
	}
	return cd.DAO.Persist()
}

// GetWrapped implements DAO interface.
func (cd *Cached) GetWrapped() DAO {
	return &Cached{cd.DAO.GetWrapped(),
		cd.accounts,
		cd.contracts,
		cd.unspents,
		cd.balances,
		cd.transfers,
		cd.storage,
	}
}

// FlushStorage flushes storage changes to the underlying DAO.
func (cd *Cached) FlushStorage() error {
	if d, ok := cd.DAO.(*Cached); ok {
		d.storage.st = cd.storage.st
		d.storage.keys = cd.storage.keys
		return nil
	}
	for h, items := range cd.storage.st {
		for _, k := range cd.storage.keys[h] {
			ti := items[k]
			switch ti.State {
			case putOp, addOp:
				err := cd.DAO.PutStorageItem(h, []byte(k), &ti.StorageItem)
				if err != nil {
					return err
				}
			case delOp:
				err := cd.DAO.DeleteStorageItem(h, []byte(k))
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func copyItem(si *state.StorageItem) *state.StorageItem {
	val := make([]byte, len(si.Value))
	copy(val, si.Value)
	return &state.StorageItem{
		Value:   val,
		IsConst: si.IsConst,
	}
}

// GetStorageItem returns StorageItem if it exists in the given store.
func (cd *Cached) GetStorageItem(scripthash util.Uint160, key []byte) *state.StorageItem {
	ti := cd.storage.getItem(scripthash, key)
	if ti != nil {
		if ti.State == delOp {
			return nil
		}
		return copyItem(&ti.StorageItem)
	}

	si := cd.DAO.GetStorageItem(scripthash, key)
	if si != nil {
		cd.storage.put(scripthash, key, getOp, si)
		return copyItem(si)
	}
	return nil
}

// PutStorageItem puts given StorageItem for given script with given
// key into the given store.
func (cd *Cached) PutStorageItem(scripthash util.Uint160, key []byte, si *state.StorageItem) error {
	item := copyItem(si)
	ti := cd.storage.getItem(scripthash, key)
	if ti != nil {
		if ti.State == delOp || ti.State == getOp {
			ti.State = putOp
		}
		ti.StorageItem = *item
		return nil
	}

	op := addOp
	if it := cd.DAO.GetStorageItem(scripthash, key); it != nil {
		op = putOp
	}
	cd.storage.put(scripthash, key, op, item)
	return nil
}

// DeleteStorageItem drops storage item for the given script with the
// given key from the store.
func (cd *Cached) DeleteStorageItem(scripthash util.Uint160, key []byte) error {
	ti := cd.storage.getItem(scripthash, key)
	if ti != nil {
		ti.State = delOp
		ti.Value = nil
		return nil
	}

	it := cd.DAO.GetStorageItem(scripthash, key)
	if it != nil {
		cd.storage.put(scripthash, key, delOp, it)
	}
	return nil
}

// StorageIteratorFunc is a function returning key-value pair or error.
type StorageIteratorFunc func() ([]byte, []byte, error)

// GetStorageItemsIterator returns iterator over all storage items.
// Function returned can be called until first error.
func (cd *Cached) GetStorageItemsIterator(hash util.Uint160, prefix []byte) (StorageIteratorFunc, error) {
	items, err := cd.DAO.GetStorageItems(hash, prefix)
	if err != nil {
		return nil, err
	}

	sort.Slice(items, func(i, j int) bool { return bytes.Compare(items[i].Key, items[j].Key) == -1 })

	cache := cd.storage.getItems(hash)

	var getItemFromCache StorageIteratorFunc
	keyIndex := -1
	getItemFromCache = func() ([]byte, []byte, error) {
		keyIndex++
		for ; keyIndex < len(cd.storage.keys[hash]); keyIndex++ {
			k := cd.storage.keys[hash][keyIndex]
			v := cache[k]
			if v.State != delOp && bytes.HasPrefix([]byte(k), prefix) {
				val := make([]byte, len(v.StorageItem.Value))
				copy(val, v.StorageItem.Value)
				return []byte(k), val, nil
			}
		}
		return nil, nil, errors.New("no more items")
	}

	var f func() ([]byte, []byte, error)
	index := -1
	f = func() ([]byte, []byte, error) {
		index++
		for ; index < len(items); index++ {
			_, ok := cache[string(items[index].Key)]
			if !ok {
				return items[index].Key, items[index].Value, nil
			}
		}
		return getItemFromCache()
	}
	return f, nil
}

// GetStorageItems returns all storage items for a given scripthash.
func (cd *Cached) GetStorageItems(hash util.Uint160, prefix []byte) ([]StorageItemWithKey, error) {
	items, err := cd.DAO.GetStorageItems(hash, prefix)
	if err != nil {
		return nil, err
	}

	cache := cd.storage.getItems(hash)
	if len(cache) == 0 {
		return items, nil
	}

	result := make([]StorageItemWithKey, 0, len(items))
	for i := range items {
		_, ok := cache[string(items[i].Key)]
		if !ok {
			result = append(result, items[i])
		}
	}
	sort.Slice(result, func(i, j int) bool { return bytes.Compare(result[i].Key, result[j].Key) == -1 })

	for _, k := range cd.storage.keys[hash] {
		v := cache[k]
		if v.State != delOp {
			val := make([]byte, len(v.StorageItem.Value))
			copy(val, v.StorageItem.Value)
			result = append(result, StorageItemWithKey{
				StorageItem: state.StorageItem{
					Value:   val,
					IsConst: v.StorageItem.IsConst,
				},
				Key: []byte(k),
			})
		}
	}

	return result, nil
}
