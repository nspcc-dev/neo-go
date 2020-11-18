package dao

import (
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCachedDaoContracts(t *testing.T) {
	store := storage.NewMemoryStore()
	pdao := NewSimple(store, netmode.UnitTestNet, false)
	dao := NewCached(pdao)

	script := []byte{0xde, 0xad, 0xbe, 0xef}
	sh := hash.Hash160(script)
	_, err := dao.GetContractState(sh)
	require.NotNil(t, err)

	m := manifest.NewManifest("Test")

	cs := &state.Contract{
		ID:       123,
		Hash:     sh,
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
	pdao := NewSimple(store, netmode.UnitTestNet, false)
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

	id := int32(random.Int(0, 1024))
	key := []byte("qwerty")
	si := &state.StorageItem{Value: []byte("poiuyt")}
	require.NoError(t, ccdao.PutStorageItem(id, key, si))
	resi := ccdao.GetStorageItem(id, key)
	assert.Equal(t, si, resi)

	resi = cdao.GetStorageItem(id, key)
	assert.Equal(t, (*state.StorageItem)(nil), resi)
	resi = pdao.GetStorageItem(id, key)
	assert.Equal(t, (*state.StorageItem)(nil), resi)

	cnt, err := ccdao.Persist()
	assert.NoError(t, err)
	assert.Equal(t, 1, cnt)
	resi = cdao.GetStorageItem(id, key)
	assert.Equal(t, si, resi)
	resi = pdao.GetStorageItem(id, key)
	assert.Equal(t, (*state.StorageItem)(nil), resi)

	cnt, err = cdao.Persist()
	assert.NoError(t, err)
	assert.Equal(t, 1, cnt)
	resi = pdao.GetStorageItem(id, key)
	assert.Equal(t, si, resi)
}
