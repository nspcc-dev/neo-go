package core

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCachedDaoAccounts(t *testing.T) {
	store := storage.NewMemoryStore()
	// Persistent DAO to check for backing storage.
	pdao := newSimpleDao(store)
	// Cached DAO.
	cdao := newCachedDao(pdao)

	hash := random.Uint160()
	_, err := cdao.GetAccountState(hash)
	require.NotNil(t, err)

	acc, err := cdao.GetAccountStateOrNew(hash)
	require.Nil(t, err)
	_, err = pdao.GetAccountState(hash)
	require.NotNil(t, err)

	acc.Version = 42
	require.NoError(t, cdao.PutAccountState(acc))
	_, err = pdao.GetAccountState(hash)
	require.NotNil(t, err)

	acc2, err := cdao.GetAccountState(hash)
	require.Nil(t, err)
	require.Equal(t, acc, acc2)

	acc2, err = cdao.GetAccountStateOrNew(hash)
	require.Nil(t, err)
	require.Equal(t, acc, acc2)

	_, err = cdao.Persist()
	require.Nil(t, err)

	acct, err := pdao.GetAccountState(hash)
	require.Nil(t, err)
	require.Equal(t, acc, acct)
}

func TestCachedDaoContracts(t *testing.T) {
	store := storage.NewMemoryStore()
	pdao := newSimpleDao(store)
	dao := newCachedDao(pdao)

	script := []byte{0xde, 0xad, 0xbe, 0xef}
	sh := hash.Hash160(script)
	_, err := dao.GetContractState(sh)
	require.NotNil(t, err)

	cs := &state.Contract{}
	cs.Name = "test"
	cs.Script = script
	cs.ParamList = []smartcontract.ParamType{1, 2}

	require.NoError(t, dao.PutContractState(cs))
	cs2, err := dao.GetContractState(sh)
	require.Nil(t, err)
	require.Equal(t, cs, cs2)

	_, err = dao.Persist()
	require.Nil(t, err)
	dao2 := newCachedDao(pdao)
	cs2, err = dao2.GetContractState(sh)
	require.Nil(t, err)
	require.Equal(t, cs, cs2)

	require.NoError(t, dao.DeleteContractState(sh))
	cs2, err = dao2.GetContractState(sh)
	require.Nil(t, err)
	require.Equal(t, cs, cs2)
	_, err = dao.GetContractState(sh)
	require.NotNil(t, err)
}

func TestCachedCachedDao(t *testing.T) {
	store := storage.NewMemoryStore()
	// Persistent DAO to check for backing storage.
	pdao := newSimpleDao(store)
	assert.NotEqual(t, store, pdao.store)
	// Cached DAO.
	cdao := newCachedDao(pdao)
	cdaoDao := cdao.dao.(*simpleDao)
	assert.NotEqual(t, store, cdaoDao.store)
	assert.NotEqual(t, pdao.store, cdaoDao.store)

	// Cached cached DAO.
	ccdao := newCachedDao(cdao)
	ccdaoDao := ccdao.dao.(*cachedDao)
	intDao := ccdaoDao.dao.(*simpleDao)
	assert.NotEqual(t, store, intDao.store)
	assert.NotEqual(t, pdao.store, intDao.store)
	assert.NotEqual(t, cdaoDao.store, intDao.store)

	hash := random.Uint160()
	key := []byte("qwerty")
	si := &state.StorageItem{Value: []byte("poiuyt")}
	require.NoError(t, ccdao.PutStorageItem(hash, key, si))
	resi := ccdao.GetStorageItem(hash, key)
	assert.Equal(t, si, resi)

	resi = cdao.GetStorageItem(hash, key)
	assert.Equal(t, (*state.StorageItem)(nil), resi)
	resi = pdao.GetStorageItem(hash, key)
	assert.Equal(t, (*state.StorageItem)(nil), resi)

	cnt, err := ccdao.Persist()
	assert.NoError(t, err)
	assert.Equal(t, 1, cnt)
	resi = cdao.GetStorageItem(hash, key)
	assert.Equal(t, si, resi)
	resi = pdao.GetStorageItem(hash, key)
	assert.Equal(t, (*state.StorageItem)(nil), resi)

	cnt, err = cdao.Persist()
	assert.NoError(t, err)
	assert.Equal(t, 1, cnt)
	resi = pdao.GetStorageItem(hash, key)
	assert.Equal(t, si, resi)
}
