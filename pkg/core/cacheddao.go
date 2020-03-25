package core

import (
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// cachedDao is a data access object that mimics dao, but has a write cache
// for accounts and read cache for contracts. These are the most frequently used
// objects in the storeBlock().
type cachedDao struct {
	dao
	accounts  map[util.Uint160]*state.Account
	contracts map[util.Uint160]*state.Contract
	unspents  map[util.Uint256]*state.UnspentCoin
	balances  map[util.Uint160]*state.NEP5Balances
	transfers map[util.Uint160]map[uint32]*state.NEP5TransferLog
}

// newCachedDao returns new cachedDao wrapping around given backing store.
func newCachedDao(backend storage.Store) *cachedDao {
	accs := make(map[util.Uint160]*state.Account)
	ctrs := make(map[util.Uint160]*state.Contract)
	unspents := make(map[util.Uint256]*state.UnspentCoin)
	balances := make(map[util.Uint160]*state.NEP5Balances)
	transfers := make(map[util.Uint160]map[uint32]*state.NEP5TransferLog)
	return &cachedDao{*newDao(backend), accs, ctrs, unspents, balances, transfers}
}

// GetAccountStateOrNew retrieves Account from cache or underlying Store
// or creates a new one if it doesn't exist.
func (cd *cachedDao) GetAccountStateOrNew(hash util.Uint160) (*state.Account, error) {
	if cd.accounts[hash] != nil {
		return cd.accounts[hash], nil
	}
	return cd.dao.GetAccountStateOrNew(hash)
}

// GetAccountState retrieves Account from cache or underlying Store.
func (cd *cachedDao) GetAccountState(hash util.Uint160) (*state.Account, error) {
	if cd.accounts[hash] != nil {
		return cd.accounts[hash], nil
	}
	return cd.dao.GetAccountState(hash)
}

// PutAccountState saves given Account in the cache.
func (cd *cachedDao) PutAccountState(as *state.Account) error {
	cd.accounts[as.ScriptHash] = as
	return nil
}

// GetContractState returns contract state from cache or underlying Store.
func (cd *cachedDao) GetContractState(hash util.Uint160) (*state.Contract, error) {
	if cd.contracts[hash] != nil {
		return cd.contracts[hash], nil
	}
	cs, err := cd.dao.GetContractState(hash)
	if err == nil {
		cd.contracts[hash] = cs
	}
	return cs, err
}

// PutContractState puts given contract state into the given store.
func (cd *cachedDao) PutContractState(cs *state.Contract) error {
	cd.contracts[cs.ScriptHash()] = cs
	return cd.dao.PutContractState(cs)
}

// DeleteContractState deletes given contract state in cache and backing Store.
func (cd *cachedDao) DeleteContractState(hash util.Uint160) error {
	cd.contracts[hash] = nil
	return cd.dao.DeleteContractState(hash)
}

// GetUnspentCoinState retrieves UnspentCoin from cache or underlying Store.
func (cd *cachedDao) GetUnspentCoinState(hash util.Uint256) (*state.UnspentCoin, error) {
	if cd.unspents[hash] != nil {
		return cd.unspents[hash], nil
	}
	return cd.dao.GetUnspentCoinState(hash)
}

// PutUnspentCoinState saves given UnspentCoin in the cache.
func (cd *cachedDao) PutUnspentCoinState(hash util.Uint256, ucs *state.UnspentCoin) error {
	cd.unspents[hash] = ucs
	return nil
}

// GetNEP5Balances retrieves NEP5Balances for the acc.
func (cd *cachedDao) GetNEP5Balances(acc util.Uint160) (*state.NEP5Balances, error) {
	if bs := cd.balances[acc]; bs != nil {
		return bs, nil
	}
	return cd.dao.GetNEP5Balances(acc)
}

// PutNEP5Balances saves NEP5Balances for the acc.
func (cd *cachedDao) PutNEP5Balances(acc util.Uint160, bs *state.NEP5Balances) error {
	cd.balances[acc] = bs
	return nil
}

// GetNEP5TransferLog retrieves NEP5TransferLog for the acc.
func (cd *cachedDao) GetNEP5TransferLog(acc util.Uint160, index uint32) (*state.NEP5TransferLog, error) {
	ts := cd.transfers[acc]
	if ts != nil && ts[index] != nil {
		return ts[index], nil
	}
	return cd.dao.GetNEP5TransferLog(acc, index)
}

// PutNEP5TransferLog saves NEP5TransferLog for the acc.
func (cd *cachedDao) PutNEP5TransferLog(acc util.Uint160, index uint32, bs *state.NEP5TransferLog) error {
	ts := cd.transfers[acc]
	if ts == nil {
		ts = make(map[uint32]*state.NEP5TransferLog, 2)
		cd.transfers[acc] = ts
	}
	ts[index] = bs
	return nil
}

// AppendNEP5Transfer appends new transfer to a transfer event log.
func (cd *cachedDao) AppendNEP5Transfer(acc util.Uint160, index uint32, tr *state.NEP5Transfer) (bool, error) {
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
func (cd *cachedDao) Persist() (int, error) {
	buf := io.NewBufBinWriter()

	for sc := range cd.accounts {
		err := cd.dao.putAccountState(cd.accounts[sc], buf)
		if err != nil {
			return 0, err
		}
		buf.Reset()
	}
	for hash := range cd.unspents {
		err := cd.dao.putUnspentCoinState(hash, cd.unspents[hash], buf)
		if err != nil {
			return 0, err
		}
		buf.Reset()
	}
	for acc, bs := range cd.balances {
		err := cd.dao.putNEP5Balances(acc, bs, buf)
		if err != nil {
			return 0, err
		}
		buf.Reset()
	}
	for acc, ts := range cd.transfers {
		for ind, lg := range ts {
			err := cd.dao.PutNEP5TransferLog(acc, ind, lg)
			if err != nil {
				return 0, err
			}
		}
	}
	return cd.dao.Persist()
}
