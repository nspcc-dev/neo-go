package core

import (
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// cachedDao is a data access object that mimics dao, but has a write cache
// for accounts and read cache for contracts. These are the most frequently used
// objects in the storeBlock().
type cachedDao struct {
	dao
	accounts  map[util.Uint160]*state.Account
	contracts map[util.Uint160]*state.Contract
}

// newCachedDao returns new cachedDao wrapping around given backing store.
func newCachedDao(backend storage.Store) *cachedDao {
	accs := make(map[util.Uint160]*state.Account)
	ctrs := make(map[util.Uint160]*state.Contract)
	return &cachedDao{*newDao(backend), accs, ctrs}
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

// Persist flushes all the changes made into the (supposedly) persistent
// underlying store.
func (cd *cachedDao) Persist() (int, error) {
	for sc := range cd.accounts {
		err := cd.dao.PutAccountState(cd.accounts[sc])
		if err != nil {
			return 0, err
		}
	}
	return cd.dao.Persist()
}
