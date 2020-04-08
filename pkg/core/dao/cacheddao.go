package dao

import (
	"errors"

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
}

// NewCached returns new Cached wrapping around given backing store.
func NewCached(d DAO) *Cached {
	accs := make(map[util.Uint160]*state.Account)
	ctrs := make(map[util.Uint160]*state.Contract)
	unspents := make(map[util.Uint256]*state.UnspentCoin)
	balances := make(map[util.Uint160]*state.NEP5Balances)
	transfers := make(map[util.Uint160]map[uint32]*state.NEP5TransferLog)
	return &Cached{d.GetWrapped(), accs, ctrs, unspents, balances, transfers}
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
	lowerCache, ok := cd.DAO.(*Cached)
	// If the lower DAO is Cached, we only need to flush the MemCached DB.
	// This actually breaks DAO interface incapsulation, but for our current
	// usage scenario it should be good enough if cd doesn't modify object
	// caches (accounts/contracts/etc) in any way.
	if ok {
		var simpleCache *Simple
		for simpleCache == nil {
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
	}
}
