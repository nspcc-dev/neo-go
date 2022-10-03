package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/storage/dbconfig"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/stretchr/testify/require"
	"go.etcd.io/bbolt"
)

func newBoltStoreForTesting(t testing.TB) Store {
	d := t.TempDir()
	testFileName := filepath.Join(d, "test_bolt_db")
	boltDBStore, err := NewBoltDBStore(dbconfig.BoltDBOptions{FilePath: testFileName})
	require.NoError(t, err)
	return boltDBStore
}

func TestROBoltDB(t *testing.T) {
	d := t.TempDir()
	testFileName := filepath.Join(d, "test_ro_bolt_db")
	cfg := dbconfig.BoltDBOptions{
		FilePath: testFileName,
		ReadOnly: true,
	}

	// If DB doesn't exist, then error should be returned.
	_, err := NewBoltDBStore(cfg)
	require.Error(t, err)

	// Create the DB and try to open it in RO mode.
	cfg.ReadOnly = false
	store, err := NewBoltDBStore(cfg)
	require.NoError(t, err)
	require.NoError(t, store.Close())
	cfg.ReadOnly = true

	store, err = NewBoltDBStore(cfg)
	require.NoError(t, err)
	// Changes must be prohibited.
	putErr := store.PutChangeSet(map[string][]byte{"one": []byte("one")}, nil)
	require.ErrorIs(t, putErr, bbolt.ErrDatabaseReadOnly)
	require.NoError(t, store.Close())

	// Create the DB without buckets and try to open it in RO mode, an error is expected.
	fileMode := os.FileMode(0600)
	cfg.FilePath = filepath.Join(d, "clean_ro_bolt_db")
	require.NoError(t, io.MakeDirForFile(cfg.FilePath, "BoltDB"))
	db, err := bbolt.Open(cfg.FilePath, fileMode, nil)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	_, err = NewBoltDBStore(cfg)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "root bucket does not exist"))
}
