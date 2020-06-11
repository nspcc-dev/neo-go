package dao

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCachedDaoAccounts(t *testing.T) {
	store := storage.NewMemoryStore()
	// Persistent DAO to check for backing storage.
	pdao := NewSimple(store)
	// Cached DAO.
	cdao := NewCached(pdao)

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
	pdao := NewSimple(store)
	dao := NewCached(pdao)

	script := []byte{0xde, 0xad, 0xbe, 0xef}
	sh := hash.Hash160(script)
	_, err := dao.GetContractState(sh)
	require.NotNil(t, err)

	m := manifest.NewManifest(hash.Hash160(script))
	m.ABI.EntryPoint.Name = "somename"
	m.ABI.EntryPoint.Parameters = []manifest.Parameter{
		manifest.NewParameter("first", smartcontract.IntegerType),
		manifest.NewParameter("second", smartcontract.StringType),
	}
	m.ABI.EntryPoint.ReturnType = smartcontract.BoolType

	cs := &state.Contract{
		ID:       123,
		Script:   script,
		Manifest: *m,
	}

	require.NoError(t, dao.PutContractState(cs))
	cs2, err := dao.GetContractState(sh)
	require.Nil(t, err)
	require.Equal(t, cs, cs2)

	_, err = dao.Persist()
	require.Nil(t, err)
	dao2 := NewCached(pdao)
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
	pdao := NewSimple(store)
	assert.NotEqual(t, store, pdao.Store)
	// Cached DAO.
	cdao := NewCached(pdao)
	cdaoDao := cdao.DAO.(*Simple)
	assert.NotEqual(t, store, cdaoDao.Store)
	assert.NotEqual(t, pdao.Store, cdaoDao.Store)

	// Cached cached DAO.
	ccdao := NewCached(cdao)
	ccdaoDao := ccdao.DAO.(*Cached)
	intDao := ccdaoDao.DAO.(*Simple)
	assert.NotEqual(t, store, intDao.Store)
	assert.NotEqual(t, pdao.Store, intDao.Store)
	assert.NotEqual(t, cdaoDao.Store, intDao.Store)

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
