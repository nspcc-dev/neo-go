package dao

import (
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCachedCachedDao(t *testing.T) {
	store := storage.NewMemoryStore()
	// Persistent DAO to check for backing storage.
	pdao := NewSimple(store, false)
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
	si := state.StorageItem("poiuyt")
	require.NoError(t, ccdao.PutStorageItem(id, key, si))
	resi := ccdao.GetStorageItem(id, key)
	assert.Equal(t, si, resi)

	resi = cdao.GetStorageItem(id, key)
	assert.Equal(t, state.StorageItem(nil), resi)
	resi = pdao.GetStorageItem(id, key)
	assert.Equal(t, state.StorageItem(nil), resi)

	cnt, err := ccdao.Persist()
	assert.NoError(t, err)
	assert.Equal(t, 1, cnt)
	resi = cdao.GetStorageItem(id, key)
	assert.Equal(t, si, resi)
	resi = pdao.GetStorageItem(id, key)
	assert.Equal(t, state.StorageItem(nil), resi)

	cnt, err = cdao.Persist()
	assert.NoError(t, err)
	assert.Equal(t, 1, cnt)
	resi = pdao.GetStorageItem(id, key)
	assert.Equal(t, si, resi)
}
