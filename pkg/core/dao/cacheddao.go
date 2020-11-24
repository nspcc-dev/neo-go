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
	contracts map[util.Uint160]*state.Contract
	balances  map[util.Uint160]*state.NEP17Balances
	transfers map[util.Uint160]map[uint32]*state.NEP17TransferLog

	dropNEP17Cache bool
}

// NewCached returns new Cached wrapping around given backing store.
func NewCached(d DAO) *Cached {
	ctrs := make(map[util.Uint160]*state.Contract)
	balances := make(map[util.Uint160]*state.NEP17Balances)
	transfers := make(map[util.Uint160]map[uint32]*state.NEP17TransferLog)
	return &Cached{d.GetWrapped(), ctrs, balances, transfers, false}
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

// GetNEP17Balances retrieves NEP17Balances for the acc.
func (cd *Cached) GetNEP17Balances(acc util.Uint160) (*state.NEP17Balances, error) {
	if bs := cd.balances[acc]; bs != nil {
		return bs, nil
	}
	return cd.DAO.GetNEP17Balances(acc)
}

// PutNEP17Balances saves NEP17Balances for the acc.
func (cd *Cached) PutNEP17Balances(acc util.Uint160, bs *state.NEP17Balances) error {
	cd.balances[acc] = bs
	return nil
}

// GetNEP17TransferLog retrieves NEP17TransferLog for the acc.
func (cd *Cached) GetNEP17TransferLog(acc util.Uint160, index uint32) (*state.NEP17TransferLog, error) {
	ts := cd.transfers[acc]
	if ts != nil && ts[index] != nil {
		return ts[index], nil
	}
	return cd.DAO.GetNEP17TransferLog(acc, index)
}

// PutNEP17TransferLog saves NEP17TransferLog for the acc.
func (cd *Cached) PutNEP17TransferLog(acc util.Uint160, index uint32, bs *state.NEP17TransferLog) error {
	ts := cd.transfers[acc]
	if ts == nil {
		ts = make(map[uint32]*state.NEP17TransferLog, 2)
		cd.transfers[acc] = ts
	}
	ts[index] = bs
	return nil
}

// AppendNEP17Transfer appends new transfer to a transfer event log.
func (cd *Cached) AppendNEP17Transfer(acc util.Uint160, index uint32, tr *state.NEP17Transfer) (bool, error) {
	lg, err := cd.GetNEP17TransferLog(acc, index)
	if err != nil {
		return false, err
	}
	if err := lg.Append(tr); err != nil {
		return false, err
	}
	return lg.Size() >= state.NEP17TransferBatchSize, cd.PutNEP17TransferLog(acc, index, lg)
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
		if cd.dropNEP17Cache {
			lowerCache.balances = make(map[util.Uint160]*state.NEP17Balances)
		}
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

	for acc, bs := range cd.balances {
		err := cd.DAO.putNEP17Balances(acc, bs, buf)
		if err != nil {
			return 0, err
		}
		buf.Reset()
	}
	for acc, ts := range cd.transfers {
		for ind, lg := range ts {
			err := cd.DAO.PutNEP17TransferLog(acc, ind, lg)
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
		cd.contracts,
		cd.balances,
		cd.transfers,
		false,
	}
}
