package dao

import (
	"errors"

	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// Cached is a data access object that mimics DAO, but has a write cache
// for accounts and NEP17 transfer data. These are the most frequently used
// objects in the storeBlock().
type Cached struct {
	DAO
	balances  map[util.Uint160]*state.NEP17TransferInfo
	transfers map[util.Uint160]map[uint32]*state.NEP17TransferLog

	dropNEP17Cache bool
}

// NewCached returns new Cached wrapping around given backing store.
func NewCached(d DAO) *Cached {
	balances := make(map[util.Uint160]*state.NEP17TransferInfo)
	transfers := make(map[util.Uint160]map[uint32]*state.NEP17TransferLog)
	return &Cached{d.GetWrapped(), balances, transfers, false}
}

// GetNEP17TransferInfo retrieves NEP17TransferInfo for the acc.
func (cd *Cached) GetNEP17TransferInfo(acc util.Uint160) (*state.NEP17TransferInfo, error) {
	if bs := cd.balances[acc]; bs != nil {
		return bs, nil
	}
	return cd.DAO.GetNEP17TransferInfo(acc)
}

// PutNEP17TransferInfo saves NEP17TransferInfo for the acc.
func (cd *Cached) PutNEP17TransferInfo(acc util.Uint160, bs *state.NEP17TransferInfo) error {
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
func (cd *Cached) AppendNEP17Transfer(acc util.Uint160, index uint32, isNew bool, tr *state.NEP17Transfer) (bool, error) {
	var lg *state.NEP17TransferLog
	if isNew {
		lg = new(state.NEP17TransferLog)
	} else {
		var err error
		lg, err = cd.GetNEP17TransferLog(acc, index)
		if err != nil {
			return false, err
		}
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
	// caches (accounts/transfer data) in any way.
	if ok {
		if cd.dropNEP17Cache {
			lowerCache.balances = make(map[util.Uint160]*state.NEP17TransferInfo)
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
		err := cd.DAO.putNEP17TransferInfo(acc, bs, buf)
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
		cd.balances,
		cd.transfers,
		false,
	}
}
